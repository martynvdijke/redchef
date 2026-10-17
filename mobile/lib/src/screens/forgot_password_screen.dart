import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../auth/auth_provider.dart';

class ForgotPasswordScreen extends ConsumerStatefulWidget {
  const ForgotPasswordScreen({super.key});
  @override
  ConsumerState<ForgotPasswordScreen> createState() => _ForgotPasswordScreenState();
}

class _ForgotPasswordScreenState extends ConsumerState<ForgotPasswordScreen> {
  final emailCtrl = TextEditingController();
  bool loading = false;
  String? error;
  String? message;

  Future<void> _submit() async {
    final email = emailCtrl.text.trim();
    if (email.isEmpty) {
      setState(() => error = 'Enter your email');
      return;
    }
    setState(() { loading = true; error = null; message = null; });
    try {
      final msg = await ref.read(authProvider.notifier).forgotPassword(email);
      setState(() => message = msg);
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['message']?.toString() ?? e.response!.data['error']?.toString()) : e.message;
      setState(() => error = msg ?? 'Failed to send reset email');
    } catch (e) {
      setState(() => error = e.toString());
    } finally {
      if (mounted) setState(() => loading = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(title: const Text('Forgot password')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Container(
            padding: const EdgeInsets.all(16),
            decoration: BoxDecoration(color: scheme.primaryContainer, borderRadius: BorderRadius.circular(16)),
            child: Row(children: [
              Icon(Icons.mail_lock_outlined, color: scheme.onPrimaryContainer),
              const SizedBox(width: 12),
              const Expanded(child: Text("We'll send a reset link if the email exists.")),
            ]),
          ),
          const SizedBox(height: 20),
          if (error != null)
            Container(padding: const EdgeInsets.all(12), decoration: BoxDecoration(color: scheme.errorContainer, borderRadius: BorderRadius.circular(12)), child: Text(error!, style: TextStyle(color: scheme.onErrorContainer))),
          if (error != null) const SizedBox(height: 12),
          if (message != null)
            Container(padding: const EdgeInsets.all(12), decoration: BoxDecoration(color: Colors.green.shade100, borderRadius: BorderRadius.circular(12)), child: Text(message!, style: TextStyle(color: Colors.green.shade900))),
          if (message != null) const SizedBox(height: 12),
          TextField(controller: emailCtrl, keyboardType: TextInputType.emailAddress, decoration: const InputDecoration(labelText: 'Email', prefixIcon: Icon(Icons.mail_outline))),
          const SizedBox(height: 16),
          FilledButton(onPressed: loading ? null : _submit, child: loading ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white)) : const Text('Send reset link')),
          const SizedBox(height: 12),
          OutlinedButton(onPressed: () => context.push('/reset'), child: const Text('Have a token? Reset password')),
          TextButton(onPressed: () => context.pop(), child: const Text('Back to login')),
        ]),
      ),
    );
  }
}
