import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../auth/auth_provider.dart';

class RegisterScreen extends ConsumerStatefulWidget {
  const RegisterScreen({super.key});
  @override
  ConsumerState<RegisterScreen> createState() => _RegisterScreenState();
}

class _RegisterScreenState extends ConsumerState<RegisterScreen> {
  final emailCtrl = TextEditingController();
  final passCtrl = TextEditingController();
  final confirmCtrl = TextEditingController();
  bool loading = false;
  String? error;

  Future<void> _register() async {
    if (passCtrl.text != confirmCtrl.text) {
      setState(() => error = 'Passwords do not match');
      return;
    }
    setState(() { loading = true; error = null; });
    try {
      await ref.read(authProvider.notifier).register(emailCtrl.text.trim(), passCtrl.text);
      if (!mounted) return;
      context.go('/');
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString() ?? e.message) : e.message;
      setState(() => error = msg ?? 'Registration failed');
    } catch (e) {
      setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Create account')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            if (error != null)
              Container(
                padding: const EdgeInsets.all(12),
                decoration: BoxDecoration(color: Theme.of(context).colorScheme.errorContainer, borderRadius: BorderRadius.circular(12)),
                child: Text(error!, style: TextStyle(color: Theme.of(context).colorScheme.onErrorContainer)),
              ),
            if (error != null) const SizedBox(height: 12),
            TextField(controller: emailCtrl, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Email', prefixIcon: Icon(Icons.mail_outline))),
            const SizedBox(height: 12),
            TextField(controller: passCtrl, obscureText: true, decoration: const InputDecoration(labelText: 'Password', prefixIcon: Icon(Icons.lock_outline))),
            const SizedBox(height: 12),
            TextField(controller: confirmCtrl, obscureText: true, decoration: const InputDecoration(labelText: 'Confirm password', prefixIcon: Icon(Icons.lock_outline))),
            const SizedBox(height: 20),
            FilledButton(
              onPressed: loading ? null : _register,
              child: loading ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white)) : const Text('Create account'),
            ),
            const SizedBox(height: 10),
            OutlinedButton(onPressed: () => context.go('/login'), child: const Text('Already have an account? Log in')),
          ],
        ),
      ),
    );
  }
}
