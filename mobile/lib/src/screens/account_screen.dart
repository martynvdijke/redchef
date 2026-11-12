import 'dart:convert';

import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:url_launcher/url_launcher.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';
import '../notifications/push_service.dart';

class AccountScreen extends ConsumerStatefulWidget {
  const AccountScreen({super.key});

  @override
  ConsumerState<AccountScreen> createState() => _AccountScreenState();
}

class _AccountScreenState extends ConsumerState<AccountScreen> {
  bool _exporting = false;
  NotificationConfig? _config;
  NotificationPrefs? _prefs;
  bool _notificationsLoading = true;
  bool _savingPrefs = false;

  @override
  void initState() {
    super.initState();
    _loadNotifications();
  }

  Future<void> _loadNotifications() async {
    if (!ref.read(authProvider).authenticated) {
      if (mounted) setState(() => _notificationsLoading = false);
      return;
    }
    final api = ref.read(apiClientProvider);
    final config = await api.getNotificationConfig();
    final prefs = await api.getNotificationPrefs();
    if (!mounted) return;
    setState(() {
      _config = config;
      _prefs = prefs;
      _notificationsLoading = false;
    });
  }

  Future<void> _savePrefs(NotificationPrefs next) async {
    final previous = _prefs;
    setState(() {
      _prefs = next;
      _savingPrefs = true;
    });
    try {
      final saved = await ref.read(apiClientProvider).updateNotificationPrefs(next);
      if (mounted) setState(() => _prefs = saved);
    } catch (_) {
      if (mounted) {
        setState(() => _prefs = previous);
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Could not save notification settings')),
        );
      }
    } finally {
      if (mounted) setState(() => _savingPrefs = false);
    }
  }

  Future<void> _toggleNewPosts(bool value) async {
    final prefs = _prefs;
    if (prefs == null) return;
    await _savePrefs(prefs.copyWith(newPosts: value));
  }

  Future<void> _togglePush(bool value) async {
    final prefs = _prefs;
    if (prefs == null) return;
    if (!value) {
      await _savePrefs(prefs.copyWith(push: false));
      return;
    }
    final granted = await ref.read(pushServiceProvider).requestPermission();
    if (!granted) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Allow notifications for RedChef in Android settings to receive push messages.')),
        );
      }
      // Without the OS permission push cannot work, so keep the stored
      // preference in line with reality.
      await _savePrefs(prefs.copyWith(push: false));
      return;
    }
    await _savePrefs(prefs.copyWith(push: true));
  }

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
          if (auth.authenticated) ...[
            _buildNotificationsCard(context),
            const SizedBox(height: 12),
          ],
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

  Widget _buildNotificationsCard(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final config = _config;
    final prefs = _prefs;
    final titleStyle = Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800);
    final hintStyle = Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant);

    final Widget body;
    if (_notificationsLoading) {
      body = const Padding(
        padding: EdgeInsets.symmetric(vertical: 12),
        child: LinearProgressIndicator(minHeight: 2),
      );
    } else if (config == null || !config.enabled) {
      body = Text('Notifications are turned off on this server.', style: hintStyle);
    } else {
      body = Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            value: prefs?.newPosts ?? true,
            onChanged: (_savingPrefs || prefs == null || !config.newPostEnabled) ? null : _toggleNewPosts,
            title: const Text('New posts'),
            subtitle: Text(
              config.newPostEnabled
                  ? 'Notify me when a new post is published'
                  : 'New post notifications are turned off on this server.',
            ),
          ),
          ValueListenableBuilder<PushState>(
            valueListenable: ref.watch(pushServiceProvider).state,
            builder: (context, pushState, _) {
              final serverReady = config.fcmEnabled;
              final deviceReady = pushState.firebaseAvailable;
              final canToggle = serverReady && deviceReady && !_savingPrefs && prefs != null;
              final String subtitle;
              if (!serverReady) {
                subtitle = 'Push is not configured on this server.';
              } else if (!deviceReady) {
                subtitle = 'Push is not available in this build.';
              } else if (pushState.permissionDenied) {
                subtitle = 'Blocked in Android settings — allow notifications for RedChef.';
              } else {
                subtitle = 'Receive new posts as Android notifications.';
              }
              return SwitchListTile(
                contentPadding: EdgeInsets.zero,
                value: (prefs?.push ?? false) && serverReady && deviceReady,
                onChanged: canToggle ? _togglePush : null,
                title: const Text('Push notifications'),
                subtitle: Text(subtitle),
              );
            },
          ),
        ],
      );
    }

    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(children: [
              Icon(Icons.notifications_outlined, color: scheme.primary),
              const SizedBox(width: 8),
              Text('Notifications', style: titleStyle),
            ]),
            const SizedBox(height: 4),
            body,
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
