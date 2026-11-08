import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'src/api/api_client.dart';
import 'src/auth/storage.dart';
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
    if (mounted) setState(() => ready = true);
    // Telemetry is best-effort and must never delay the first frame.
    unawaited(_startTelemetry());
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
      theme: buildTheme(Brightness.light),
      darkTheme: buildTheme(Brightness.dark),
      themeMode: ThemeMode.system,
      routerConfig: router,
    );
  }
}
