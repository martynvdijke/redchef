import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../auth/auth_provider.dart';

class OnboardingScreen extends ConsumerStatefulWidget {
  const OnboardingScreen({super.key});
  @override
  ConsumerState<OnboardingScreen> createState() => _OnboardingScreenState();
}

class _OnboardingScreenState extends ConsumerState<OnboardingScreen> {
  final ctrl = TextEditingController();
  bool validating = false;
  String? error;

  @override
  void initState() {
    super.initState();
    final base = ref.read(apiClientProvider).baseUrl;
    if (base != null) ctrl.text = base;
  }

  Future<void> _save() async {
    final url = ctrl.text.trim();
    if (url.isEmpty) {
      setState(() => error = 'Enter your RedChef server URL');
      return;
    }
    var normalized = url;
    if (!normalized.startsWith('http')) normalized = 'https://$normalized';
    setState(() { validating = true; error = null; });
    final err = await ref.read(authProvider.notifier).validateServerUrlDetailed(normalized);
    if (!mounted) return;
    if (err != null) {
      setState(() { validating = false; error = err; });
      return;
    }
    await ref.read(authProvider.notifier).setBaseUrl(normalized);
    setState(() => validating = false);
    if (!mounted) return;
    context.go('/login');
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(24),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: 24),
              Container(
                padding: const EdgeInsets.all(18),
                decoration: BoxDecoration(
                  color: scheme.primaryContainer,
                  borderRadius: BorderRadius.circular(20),
                ),
                child: Row(
                  children: [
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(color: scheme.primary, borderRadius: BorderRadius.circular(14)),
                      child: const Icon(Icons.restaurant, color: Colors.white, size: 28),
                    ),
                    const SizedBox(width: 14),
                    Expanded(
                      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                        Text('RedChef', style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w900)),
                        Text('Connect to your server to start feasting', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onPrimaryContainer)),
                      ]),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 28),
              Text('Server URL', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
              const SizedBox(height: 8),
              TextField(
                controller: ctrl,
                keyboardType: TextInputType.url,
                autocorrect: false,
                decoration: InputDecoration(
                  hintText: 'https://redchef.example.com',
                  prefixIcon: const Icon(Icons.link),
                  errorText: error,
                ),
                onSubmitted: (_) => _save(),
              ),
              const SizedBox(height: 12),
              Text('Enter the base URL of your RedChef instance (without trailing slash). We validate via /api/setup/status.',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
              const SizedBox(height: 20),
              FilledButton(
                onPressed: validating ? null : _save,
                child: validating
                    ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white))
                    : const Text('Continue'),
              ),
              const SizedBox(height: 16),
              OutlinedButton(
                onPressed: () {
                  ctrl.text = 'http://10.0.2.2:8080';
                  setState(() {});
                },
                child: const Text('Use emulator default (10.0.2.2:8080)'),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
