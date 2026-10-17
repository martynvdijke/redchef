import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../auth/auth_provider.dart';

class ResetPasswordScreen extends ConsumerStatefulWidget {
  const ResetPasswordScreen({super.key, this.initialToken});
  final String? initialToken;
  @override
  ConsumerState<ResetPasswordScreen> createState() => _ResetPasswordScreenState();
}

class _ResetPasswordScreenState extends ConsumerState<ResetPasswordScreen> {
  late final tokenCtrl = TextEditingController(text: widget.initialToken ?? '');
  final passCtrl = TextEditingController();
  final confirmCtrl = TextEditingController();
  bool loading = false;
  bool obscure = true;
  String? error;
  String? message;

  Future<void> _submit() async {
    final token = tokenCtrl.text.trim();
    final pass = passCtrl.text;
    final confirm = confirmCtrl.text;
    if (token.isEmpty) { setState(() => error = 'Enter the reset token'); return; }
    if (pass.isEmpty) { setState(() => error = 'Enter a new password'); return; }
    if (pass != confirm) { setState(() => error = 'Passwords do not match'); return; }
    setState(() { loading = true; error = null; message = null; });
    try {
      final msg = await ref.read(authProvider.notifier).resetPassword(token: token, password: pass, confirmPassword: confirm);
      setState(() => message = msg);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg)));
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['message']?.toString() ?? e.response!.data['error']?.toString()) : e.message;
      setState(() => error = msg ?? 'Reset failed');
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
      appBar: AppBar(title: const Text('Reset password')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
          Text('Paste the token from your email. Deep links are not required — manual entry is fine.', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
          const SizedBox(height: 16),
          if (error != null)
            Container(padding: const EdgeInsets.all(12), decoration: BoxDecoration(color: scheme.errorContainer, borderRadius: BorderRadius.circular(12)), child: Text(error!, style: TextStyle(color: scheme.onErrorContainer))),
          if (error != null) const SizedBox(height: 12),
          if (message != null)
            Container(padding: const EdgeInsets.all(12), decoration: BoxDecoration(color: Colors.green.shade100, borderRadius: BorderRadius.circular(12)), child: Text(message!, style: TextStyle(color: Colors.green.shade900))),
          if (message != null) const SizedBox(height: 12),
          TextField(controller: tokenCtrl, decoration: const InputDecoration(labelText: 'Reset token *', prefixIcon: Icon(Icons.key_outlined), hintText: 'Token from email')),
          const SizedBox(height: 12),
          TextField(controller: passCtrl, obscureText: obscure, decoration: InputDecoration(labelText: 'New password *', prefixIcon: const Icon(Icons.lock_outline), suffixIcon: IconButton(icon: Icon(obscure ? Icons.visibility_off : Icons.visibility), onPressed: () => setState(() => obscure = !obscure)))),
          const SizedBox(height: 12),
          TextField(controller: confirmCtrl, obscureText: obscure, decoration: const InputDecoration(labelText: 'Confirm password', prefixIcon: Icon(Icons.lock_outline))),
          const SizedBox(height: 20),
          FilledButton(onPressed: loading ? null : _submit, child: loading ? const SizedBox(height: 18, width: 18, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white)) : const Text('Reset password')),
          const SizedBox(height: 12),
          OutlinedButton(onPressed: () => context.go('/login'), child: const Text('Back to login')),
        ]),
      ),
    );
  }
}
