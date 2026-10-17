import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';
import '../auth/auth_provider.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});
  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final emailCtrl = TextEditingController();
  final passCtrl = TextEditingController();
  bool loading = false;
  String? error;
  bool oidcEnabled = false;

  @override
  void initState() {
    super.initState();
    _checkOidc();
  }

  Future<void> _checkOidc() async {
    final v = await ref.read(authProvider.notifier).checkOidcEnabled();
    if (mounted) setState(() => oidcEnabled = v);
  }

  Future<void> _login() async {
    setState(() { loading = true; error = null; });
    try {
      await ref.read(authProvider.notifier).login(emailCtrl.text.trim(), passCtrl.text);
      if (!mounted) return;
      context.go('/');
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString() ?? e.message) : e.message;
      setState(() => error = msg ?? 'Login failed');
    } catch (e) {
      setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  Future<void> _oidc() async {
    final base = ref.read(apiClientProvider).baseUrl;
    if (base == null) return;
    final uri = Uri.parse('$base/api/auth/oidc/login');
    await launchUrl(uri, mode: LaunchMode.externalApplication);
    if (!mounted) return;
    ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Opened browser — complete login there, then return and pull to refresh or tap “I\'ve logged in”')));
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Welcome back'),
        actions: [
          TextButton(onPressed: () => context.go('/onboarding'), child: const Text('Change server')),
        ],
      ),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(color: scheme.primaryContainer, borderRadius: BorderRadius.circular(16)),
              child: Row(children: [
                Icon(Icons.soup_kitchen, color: scheme.onPrimaryContainer, size: 32),
                const SizedBox(width: 12),
                Expanded(child: Text('Sign in to RedChef', style: Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800))),
              ]),
            ),
            const SizedBox(height: 20),
            if (error != null)
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(color: scheme.errorContainer, borderRadius: BorderRadius.circular(12)),
                child: Text(error!, style: TextStyle(color: scheme.onErrorContainer)),
              ),
            if (error != null) const SizedBox(height: 12),
            TextField(
              controller: emailCtrl,
              keyboardType: TextInputType.emailAddress,
              decoration: const InputDecoration(labelText: 'Email', prefixIcon: Icon(Icons.mail_outline)),
            ),
            const SizedBox(height: 12),
            TextField(
              controller: passCtrl,
              obscureText: true,
              decoration: const InputDecoration(labelText: 'Password', prefixIcon: Icon(Icons.lock_outline)),
              onSubmitted: (_) => _login(),
            ),
            const SizedBox(height: 20),
            FilledButton(
              onPressed: loading ? null : _login,
              child: loading ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white)) : const Text('Log in'),
            ),
            Align(alignment: Alignment.centerRight, child: TextButton(onPressed: () => context.push('/forgot'), child: const Text('Forgot password?'))),
            const SizedBox(height: 6),
            OutlinedButton(onPressed: () => context.go('/register'), child: const Text('Create account')),
            TextButton(onPressed: () => context.push('/reset'), child: const Text('Have a reset token? Reset password')),
            if (oidcEnabled) ...[
              const SizedBox(height: 16),
              const Row(children: [Expanded(child: Divider()), Padding(padding: EdgeInsets.symmetric(horizontal: 12), child: Text('or')), Expanded(child: Divider())]),
              const SizedBox(height: 16),
              OutlinedButton.icon(
                onPressed: _oidc,
                icon: const Icon(Icons.shield_outlined),
                label: const Text('Login with Authelia'),
                style: OutlinedButton.styleFrom(padding: const EdgeInsets.symmetric(vertical: 14)),
              ),
              const SizedBox(height: 8),
              TextButton(
                onPressed: () async {
                  await ref.read(authProvider.notifier).refreshMe();
                  if (context.mounted && ref.read(authProvider).authenticated) {
                    context.go('/');
                  }
                },
                child: const Text("I've logged in — refresh"),
              ),
            ],
            const SizedBox(height: 12),
            TextButton(onPressed: () => context.go('/'), child: const Text('Continue as guest')),
          ],
        ),
      ),
    );
  }
}
