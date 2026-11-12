import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'src/api/api_client.dart';
import 'src/auth/storage.dart';
import 'src/notifications/push_service.dart';
import 'src/telemetry/telemetry.dart';
import 'src/theme.dart';
import 'src/auth/auth_provider.dart';
import 'src/routing/router.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  runApp(const ProviderScope(child: RedChefApp()));
}

class RedChefApp extends ConsumerStatefulWidget {
  const RedChefApp({super.key});
  @override
  ConsumerState<RedChefApp> createState() => _RedChefAppState();
}

class _RedChefAppState extends ConsumerState<RedChefApp> {
  bool ready = false;

  @override
  void initState() {
    super.initState();
    _init();
  }

  Future<void> _init() async {
    await ref.read(authProvider.notifier).init();
    final authenticated = ref.read(authProvider).authenticated;
    if (mounted) setState(() => ready = true);
    // Telemetry is best-effort and must never delay the first frame.
    unawaited(_startTelemetry());
    if (authenticated) {
      // Already signed in from a previous run: register this device for push.
      unawaited(ref.read(pushServiceProvider).onAuthenticated());
    }
    // Push needs the GoRouter instance, which only exists once routing is live.
    WidgetsBinding.instance.addPostFrameCallback((_) => _wirePush());
  }

  /// Connects notification taps to the router and replays one that arrived
  /// before the app was ready.
  void _wirePush() {
    final push = ref.read(pushServiceProvider);
    push.onOpenRoute = (route) => ref.read(routerProvider).go(route);
    push.takePendingRoute();
  }

  /// Resolves the server the user is actually talking to, then asks it whether
  /// OTLP export is configured. Offline or slow servers just leave telemetry
  /// disabled; nothing here can throw into the widget tree.
  Future<void> _startTelemetry() async {
    String baseUrl = defaultRedChefBaseUrl;
    try {
      final stored = await SecureStore.getBaseUrl();
      if (stored != null && stored.trim().isNotEmpty) baseUrl = stored;
    } catch (_) {
      // Secure storage unavailable: fall back to the hosted instance.
    }
    await Telemetry.instance.startFromServer(baseUrl);
  }

  @override
  Widget build(BuildContext context) {
    // Start/stop push as the session comes and goes. Registration needs an
    // authenticated ApiClient (session cookie), so it waits for sign-in.
    ref.listen<AuthState>(authProvider, (previous, next) {
      final wasAuthenticated = previous?.authenticated ?? false;
      final push = ref.read(pushServiceProvider);
      if (!wasAuthenticated && next.authenticated) {
        unawaited(push.onAuthenticated());
      } else if (wasAuthenticated && !next.authenticated) {
        unawaited(push.unregisterOnLogout());
      }
    });

    if (!ready) {
      return MaterialApp(
        debugShowCheckedModeBanner: false,
        theme: buildTheme(Brightness.light),
        home: const Scaffold(body: Center(child: CircularProgressIndicator())),
      );
    }
    final router = ref.watch(routerProvider);
    return MaterialApp.router(
      title: 'RedChef',
      debugShowCheckedModeBanner: false,
      scaffoldMessengerKey: appMessengerKey,
      theme: buildTheme(Brightness.light),
      darkTheme: buildTheme(Brightness.dark),
      themeMode: ThemeMode.system,
      routerConfig: router,
    );
  }
}
