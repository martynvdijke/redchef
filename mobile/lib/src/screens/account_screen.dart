import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';
import '../auth/auth_provider.dart';

class AccountScreen extends ConsumerStatefulWidget {
  const AccountScreen({super.key});

  @override
  ConsumerState<AccountScreen> createState() => _AccountScreenState();
}

class _AccountScreenState extends ConsumerState<AccountScreen> {
  bool _exporting = false;

  @override
  Widget build(BuildContext context) {
    final auth = ref.watch(authProvider);
    final scheme = Theme.of(context).colorScheme;
    final base = ref.watch(apiClientProvider).baseUrl ?? '—';
    return Scaffold(
      appBar: AppBar(title: const Text('Account')),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                Row(children: [Icon(Icons.person_outline, color: scheme.primary), const SizedBox(width: 8), Text('Profile', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800))]),
                const SizedBox(height: 12),
                Text('Email: ${auth.me?.email ?? '—'}'),
                Text('Role: ${auth.me?.role ?? '—'}'),
                Text('Authenticated: ${auth.authenticated}'),
                const SizedBox(height: 8),
                Text('Server: $base', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
              ]),
            ),
          ),
          const SizedBox(height: 12),
          ListTile(leading: const Icon(Icons.restaurant_outlined), title: const Text('Admin posts'), subtitle: const Text('Drafts & published (admin only)'), enabled: auth.isAdmin, onTap: auth.isAdmin ? () => context.push('/admin/posts') : null),
          ListTile(leading: const Icon(Icons.key_outlined), title: const Text('API tokens'), subtitle: const Text('Create, rotate, revoke'), onTap: () => context.push('/tokens')),
          const Divider(),
          ListTile(leading: const Icon(Icons.privacy_tip_outlined), title: const Text('Privacy policy'), onTap: () => _open('$base/privacy')),
          ListTile(leading: const Icon(Icons.description_outlined), title: const Text('Terms of use'), onTap: () => _open('$base/terms')),
          ListTile(
            leading: _exporting
                ? const SizedBox(width: 24, height: 24, child: CircularProgressIndicator(strokeWidth: 2))
                : const Icon(Icons.download_outlined),
            title: const Text('Download my data'),
            subtitle: const Text('Export your profile, comments, favourites, tips and purchases'),
            enabled: !_exporting,
            onTap: _exporting ? null : _exportAccount,
          ),
          ListTile(leading: const Icon(Icons.delete_forever_outlined, color: Colors.red), title: const Text('Delete account', style: TextStyle(color: Colors.red)), subtitle: const Text('Permanently close your account'), onTap: () => _deleteAccount(context, ref, auth.me?.email)),
          const Divider(),
          ListTile(leading: const Icon(Icons.link_outlined), title: const Text('Change server'), onTap: () => context.go('/onboarding')),
          ListTile(leading: const Icon(Icons.logout, color: Colors.red), title: const Text('Log out', style: TextStyle(color: Colors.red)), onTap: () async { await ref.read(authProvider.notifier).logout(); if (context.mounted) context.go('/login'); }),
        ],
      ),
    ),
  );
  }

  Future<void> _open(String url) async {
    final uri = Uri.tryParse(url);
    if (uri != null) {
      await launchUrl(uri, mode: LaunchMode.externalApplication);
    }
  }

  Future<void> _exportAccount() async {
    if (_exporting) return;
    setState(() => _exporting = true);
    try {
      final json = await ref.read(apiClientProvider).exportAccount();
      if (!mounted) return;
      setState(() => _exporting = false);
      final bytes = utf8.encode(json).length;
      await showDialog<void>(
        context: context,
        builder: (ctx) => AlertDialog(
          title: const Text('My data'),
          content: SizedBox(
            width: double.maxFinite,
            height: 360,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('${json.length} characters · $bytes bytes', style: Theme.of(ctx).textTheme.bodySmall),
                const SizedBox(height: 8),
                Expanded(
                  child: SingleChildScrollView(
                    child: SelectableText(json, style: const TextStyle(fontFamily: 'monospace', fontSize: 12)),
                  ),
                ),
              ],
            ),
          ),
          actions: [
            TextButton(
              onPressed: () async {
                await Clipboard.setData(ClipboardData(text: json));
                if (ctx.mounted) {
                  ScaffoldMessenger.of(ctx).showSnackBar(const SnackBar(content: Text('Copied to clipboard')));
                }
              },
              child: const Text('Copy'),
            ),
            TextButton(onPressed: () => Navigator.of(ctx).pop(), child: const Text('Close')),
          ],
        ),
      );
    } on DioException catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Could not export data: ${e.message}')));
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Could not export data: $e')));
      }
    } finally {
      if (mounted) setState(() => _exporting = false);
    }
  }

  Future<void> _deleteAccount(BuildContext context, WidgetRef ref, String? email) async {
    final controller = TextEditingController();
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Delete account'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            const Text('This permanently deletes your account, comments, favourites, tips and purchases. Type your email address to confirm.'),
            const SizedBox(height: 12),
            TextField(controller: controller, decoration: InputDecoration(labelText: 'Email', hintText: email)),
          ],
        ),
        actions: [
          TextButton(onPressed: () => Navigator.of(ctx).pop(false), child: const Text('Cancel')),
          FilledButton(onPressed: () => Navigator.of(ctx).pop(true), child: const Text('Delete')),
        ],
      ),
    );
    if (confirmed != true) return;
    try {
      await ref.read(authProvider.notifier).deleteAccount(controller.text.trim());
      if (context.mounted) context.go('/login');
    } catch (e) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Could not delete account: $e')));
      }
    }
  }
}
