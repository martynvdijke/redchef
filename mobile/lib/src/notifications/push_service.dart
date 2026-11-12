import 'dart:async';

import 'package:firebase_core/firebase_core.dart';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../auth/auth_provider.dart';

/// Messenger used to surface foreground notifications. [PushService] has no
/// widget context of its own, so it reaches the app's ScaffoldMessenger here.
final GlobalKey<ScaffoldMessengerState> appMessengerKey = GlobalKey<ScaffoldMessengerState>();

/// Maps a server notification payload onto an in-app route.
///
/// The server sends `url: "/posts/123"` (the web path); the app's detail screen
/// lives at `/post/:id`. Missing/unknown URLs fall back to `post_id`.
String? routeForNotification(Map<String, dynamic> data) {
  final raw = (data['url'] ?? '').toString().trim();
  if (raw.isNotEmpty) {
    final posts = RegExp(r'^/?posts/(\d+)').firstMatch(raw);
    if (posts != null) return '/post/${posts.group(1)}';
    final post = RegExp(r'^/?post/(\d+)').firstMatch(raw);
    if (post != null) return '/post/${post.group(1)}';
    if (raw.startsWith('/') && !raw.startsWith('//')) return raw;
  }
  final postId = int.tryParse((data['post_id'] ?? '').toString());
  if (postId != null) return '/post/$postId';
  return null;
}

/// Observable state of the push subsystem.
@immutable
class PushState {
  /// Firebase initialised successfully (i.e. google-services.json is present).
  final bool firebaseAvailable;

  /// The OS notification permission is granted.
  final bool permissionGranted;

  /// The user denied the OS notification permission.
  final bool permissionDenied;

  /// The FCM token currently registered with the server, when known.
  final String? token;

  /// Human-readable reason push is unavailable, for diagnostics/UI.
  final String? error;

  const PushState({
    this.firebaseAvailable = false,
    this.permissionGranted = false,
    this.permissionDenied = false,
    this.token,
    this.error,
  });

  bool get canReceive => firebaseAvailable && permissionGranted;

  PushState copyWith({
    bool? firebaseAvailable,
    bool? permissionGranted,
    bool? permissionDenied,
    String? token,
    String? error,
  }) =>
      PushState(
        firebaseAvailable: firebaseAvailable ?? this.firebaseAvailable,
        permissionGranted: permissionGranted ?? this.permissionGranted,
        permissionDenied: permissionDenied ?? this.permissionDenied,
        token: token ?? this.token,
        error: error ?? this.error,
      );
}

/// Owns Firebase Messaging: initialisation, permission, token registration and
/// navigation from notification taps.
///
/// Everything is defensive: builds without `google-services.json` (or without
/// the native plugin, as in tests) simply report [PushState.firebaseAvailable]
/// as false instead of throwing into the widget tree.
class PushService {
  PushService(this._ref);

  final Ref _ref;
  final ValueNotifier<PushState> state = ValueNotifier(const PushState());

  bool _started = false;
  StreamSubscription<RemoteMessage>? _foregroundSub;
  StreamSubscription<RemoteMessage>? _openedSub;
  StreamSubscription<String>? _tokenRefreshSub;

  String? _registeredToken;
  String? _pendingRoute;

  /// Called with an in-app route when a notification is tapped. Set by main.dart
  /// once the router exists.
  void Function(String route)? onOpenRoute;

  /// Initialises Firebase and the message listeners. Safe to call repeatedly.
  Future<void> initialize() async {
    if (_started) return;
    _started = true;
    try {
      if (Firebase.apps.isEmpty) {
        await Firebase.initializeApp();
      }
      final messaging = FirebaseMessaging.instance;
      // Foreground messages are shown by the app itself (SnackBar), not by the OS.
      await messaging.setForegroundNotificationPresentationOptions(
        alert: false,
        badge: false,
        sound: false,
      );
      _foregroundSub ??= FirebaseMessaging.onMessage.listen(_onForegroundMessage);
      _openedSub ??= FirebaseMessaging.onMessageOpenedApp.listen(_onMessageOpened);
      // ignore: deprecated_member_use
      _tokenRefreshSub ??= messaging.onTokenRefresh.listen((token) => unawaited(_register(token)));

      final settings = await messaging.getNotificationSettings();
      final status = settings.authorizationStatus;
      final granted = status == AuthorizationStatus.authorized || status == AuthorizationStatus.provisional;
      state.value = state.value.copyWith(
        firebaseAvailable: true,
        permissionGranted: granted,
        permissionDenied: status == AuthorizationStatus.denied,
        error: '',
      );
      if (granted) await _fetchAndRegister();

      final initial = await messaging.getInitialMessage();
      if (initial != null) _onMessageOpened(initial);
    } catch (error) {
      // Missing google-services.json, no native plugin (tests), unsupported
      // platform: the feature degrades to disabled instead of crashing.
      _started = false;
      state.value = PushState(firebaseAvailable: false, error: _describe(error));
      debugPrint('Push notifications unavailable: $error');
    }
  }

  /// Asks for the OS notification permission and, when granted, registers the
  /// device with the server. Returns whether notifications may now be shown.
  Future<bool> requestPermission() async {
    if (!state.value.firebaseAvailable) {
      // Firebase may not have been set up yet (e.g. user toggled push early).
      await initialize();
      if (!state.value.firebaseAvailable) return false;
    }
    try {
      final settings = await FirebaseMessaging.instance.requestPermission(
        alert: true,
        badge: true,
        sound: true,
      );
      final status = settings.authorizationStatus;
      final granted = status == AuthorizationStatus.authorized || status == AuthorizationStatus.provisional;
      state.value = state.value.copyWith(
        permissionGranted: granted,
        permissionDenied: !granted,
        error: '',
      );
      if (granted) await _fetchAndRegister();
      return granted;
    } catch (error) {
      state.value = state.value.copyWith(permissionGranted: false, error: _describe(error));
      return false;
    }
  }

  /// Called once a user is signed in: initialise and (if permission is already
  /// granted) make sure this device is registered for that user.
  Future<void> onAuthenticated() async {
    await initialize();
    if (state.value.permissionGranted) {
      await _fetchAndRegister();
    }
  }

  /// Called on logout: detach this device from the departing user so it stops
  /// receiving their notifications, and drop listeners.
  Future<void> unregisterOnLogout() async {
    final token = _registeredToken ?? state.value.token;
    if (token != null && token.isNotEmpty) {
      try {
        await _ref.read(apiClientProvider).unregisterDeviceToken(token);
      } catch (_) {
        // Offline logout still proceeds; the server prunes stale tokens on send.
      }
    }
    try {
      // ignore: deprecated_member_use
      await FirebaseMessaging.instance.deleteToken();
    } catch (_) {
      // Not initialised / no plugin: nothing to delete.
    }
    _registeredToken = null;
    state.value = PushState(
      firebaseAvailable: state.value.firebaseAvailable,
      error: state.value.error,
    );
  }

  /// Replays a tap that arrived before the router was ready.
  void takePendingRoute() {
    final route = _pendingRoute;
    if (route == null) return;
    _pendingRoute = null;
    _openRoute(route);
  }

  void dispose() {
    _foregroundSub?.cancel();
    _openedSub?.cancel();
    _tokenRefreshSub?.cancel();
    _foregroundSub = null;
    _openedSub = null;
    _tokenRefreshSub = null;
    state.dispose();
  }

  Future<void> _fetchAndRegister() async {
    try {
      // ignore: deprecated_member_use
      final token = await FirebaseMessaging.instance.getToken();
      if (token != null && token.isNotEmpty) await _register(token);
    } catch (error) {
      debugPrint('Could not fetch FCM token: $error');
    }
  }

  Future<void> _register(String token) async {
    if (token.isEmpty) return;
    try {
      await _ref.read(apiClientProvider).registerDeviceToken(token);
      _registeredToken = token;
      state.value = state.value.copyWith(token: token);
    } catch (error) {
      debugPrint('Could not register device token: $error');
    }
  }

  void _onForegroundMessage(RemoteMessage message) {
    final title = message.notification?.title ?? message.data['title']?.toString() ?? '';
    final body = message.notification?.body ?? message.data['body']?.toString() ?? '';
    final text = [title, body].where((part) => part.trim().isNotEmpty).join('\n');
    if (text.isEmpty) return;
    final route = routeForNotification(message.data);
    final messenger = appMessengerKey.currentState;
    messenger?.showSnackBar(
      SnackBar(
        content: Text(text),
        duration: const Duration(seconds: 6),
        action: route == null
            ? null
            : SnackBarAction(label: 'View', onPressed: () => _openRoute(route)),
      ),
    );
  }

  void _onMessageOpened(RemoteMessage message) {
    final route = routeForNotification(message.data);
    if (route == null) return;
    _openRoute(route);
  }

  void _openRoute(String route) {
    final navigate = onOpenRoute;
    if (navigate == null) {
      _pendingRoute = route;
      return;
    }
    try {
      navigate(route);
    } catch (error) {
      debugPrint('Could not open notification route $route: $error');
      _pendingRoute = route;
    }
  }

  String _describe(Object error) {
    final text = error.toString();
    if (text.contains('FirebaseOptions') ||
        text.contains('google-services') ||
        text.contains('FirebaseApp') ||
        text.contains('MissingPluginException')) {
      return 'Push notifications are not configured for this build.';
    }
    return 'Push notifications are unavailable.';
  }
}

final pushServiceProvider = Provider<PushService>((ref) {
  final service = PushService(ref);
  ref.onDispose(service.dispose);
  return service;
});
