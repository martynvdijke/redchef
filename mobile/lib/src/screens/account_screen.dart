import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../auth/auth_provider.dart';

class AccountScreen extends ConsumerWidget {
  const AccountScreen({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final auth = ref.watch(authProvider);
    final scheme = Theme.of(context).colorScheme;
    final base = ref.watch(apiClientProvider).baseUrl ?? '—';
    return Scaffold(
      appBar: AppBar(title: const Text('Account')),
      body: ListView(
        padding: const EdgeInsets.all(16),
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
          ListTile(leading: const Icon(Icons.link_outlined), title: const Text('Change server'), onTap: () => context.go('/onboarding')),
          ListTile(leading: const Icon(Icons.logout, color: Colors.red), title: const Text('Log out', style: TextStyle(color: Colors.red)), onTap: () async { await ref.read(authProvider.notifier).logout(); if (context.mounted) context.go('/login'); }),
        ],
      ),
    );
  }
}
