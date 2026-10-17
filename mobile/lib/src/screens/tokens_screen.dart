import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:intl/intl.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';

class TokensScreen extends ConsumerStatefulWidget {
  const TokensScreen({super.key});
  @override
  ConsumerState<TokensScreen> createState() => _TokensScreenState();
}

class _TokensScreenState extends ConsumerState<TokensScreen> {
  List<ApiToken> tokens = [];
  bool loading = true;
  String? error;
  String? oneTimeToken;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { loading = true; error = null; });
    try {
      final list = await ref.read(authProvider.notifier).fetchTokens();
      setState(() { tokens = list; loading = false; });
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString() ?? e.response!.data['message']?.toString()) : e.message;
      setState(() { loading = false; error = msg ?? 'Failed to load tokens'; });
    } catch (e) {
      setState(() { loading = false; error = e.toString(); });
    }
  }

  Future<void> _create() async {
    final ctrl = TextEditingController();
    final name = await showDialog<String>(context: context, builder: (ctx) => AlertDialog(
      title: const Text('New API token'),
      content: TextField(controller: ctrl, decoration: const InputDecoration(labelText: 'Token name', hintText: 'e.g. CLI')),
      actions: [TextButton(onPressed: () => Navigator.pop(ctx), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(ctx, ctrl.text.trim()), child: const Text('Create'))],
    ));
    if (name == null || name.isEmpty) return;
    try {
      final t = await ref.read(authProvider.notifier).createToken(name);
      setState(() => oneTimeToken = t.token);
      if (!mounted) return;
      await showDialog(context: context, builder: (ctx) => AlertDialog(
        title: const Text('Token created'),
        content: Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.start, children: [
          const Text('Copy this token now — it will not be shown again:'),
          const SizedBox(height: 12),
          SelectableText(t.token ?? '', style: const TextStyle(fontFamily: 'monospace', fontWeight: FontWeight.w700)),
          const SizedBox(height: 12),
          FilledButton.tonal(onPressed: () { Clipboard.setData(ClipboardData(text: t.token ?? '')); ScaffoldMessenger.of(ctx).showSnackBar(const SnackBar(content: Text('Copied'))); }, child: const Text('Copy')),
        ]),
        actions: [FilledButton(onPressed: () => Navigator.pop(ctx), child: const Text('Done'))],
      ));
      _load();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString() ?? e.response!.data['message']?.toString()) : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Create failed')));
    }
  }

  Future<void> _revoke(ApiToken t) async {
    final ok = await showDialog<bool>(context: context, builder: (ctx) => AlertDialog(title: const Text('Revoke token?'), content: Text('Revoke "${t.name}"?'), actions: [TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Revoke'))]));
    if (ok != true) return;
    try {
      await ref.read(authProvider.notifier).revokeToken(t.id);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Revoked')));
      _load();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString()) : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Revoke failed')));
    }
  }

  Future<void> _rotate(ApiToken t) async {
    try {
      final nt = await ref.read(authProvider.notifier).rotateToken(t.id);
      if (!mounted) return;
      await showDialog(context: context, builder: (ctx) => AlertDialog(
        title: const Text('Token rotated'),
        content: Column(mainAxisSize: MainAxisSize.min, children: [const Text('New token (copy now):'), const SizedBox(height: 12), SelectableText(nt.token ?? '', style: const TextStyle(fontFamily: 'monospace', fontWeight: FontWeight.w700))]),
        actions: [FilledButton(onPressed: () => Navigator.pop(ctx), child: const Text('Done')), TextButton(onPressed: () { Clipboard.setData(ClipboardData(text: nt.token ?? '')); ScaffoldMessenger.of(ctx).showSnackBar(const SnackBar(content: Text('Copied'))); }, child: const Text('Copy'))],
      ));
      _load();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Rotate failed')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(title: const Text('API tokens')),
      floatingActionButton: FloatingActionButton.extended(onPressed: _create, icon: const Icon(Icons.add), label: const Text('New token')),
      body: loading ? const Center(child: CircularProgressIndicator()) : error != null ? Center(child: Column(mainAxisSize: MainAxisSize.min, children: [Text(error!), const SizedBox(height: 12), FilledButton(onPressed: _load, child: const Text('Retry'))])) : tokens.isEmpty ? Center(child: Column(mainAxisSize: MainAxisSize.min, children: [Icon(Icons.key_off_outlined, size: 48, color: scheme.onSurfaceVariant), const SizedBox(height: 8), const Text('No tokens yet'), const SizedBox(height: 8), FilledButton(onPressed: _create, child: const Text('Create token'))])) : RefreshIndicator(onRefresh: _load, child: ListView.separated(
        padding: const EdgeInsets.all(12),
        itemCount: tokens.length,
        separatorBuilder: (_, __) => const SizedBox(height: 8),
        itemBuilder: (_, i) {
          final t = tokens[i];
          final revoked = t.isRevoked;
          return Card(
            child: ListTile(
              title: Text(t.name, style: TextStyle(decoration: revoked ? TextDecoration.lineThrough : null, fontWeight: FontWeight.w700)),
              subtitle: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                const SizedBox(height: 4),
                Text('Created: ${t.createdAt != null ? DateFormat.yMMMd().add_jm().format(t.createdAt!) : '—'}', style: Theme.of(context).textTheme.bodySmall),
                if (t.lastUsedAt != null) Text('Last used: ${DateFormat.yMMMd().add_jm().format(t.lastUsedAt!)}', style: Theme.of(context).textTheme.bodySmall),
                if (revoked) Text('Revoked: ${t.revokedAt != null ? DateFormat.yMMMd().format(t.revokedAt!) : ''}', style: TextStyle(color: scheme.error, fontWeight: FontWeight.w600)),
              ]),
              trailing: PopupMenuButton<String>(
                onSelected: (v) { if (v == 'revoke') _revoke(t); if (v == 'rotate') _rotate(t); },
                itemBuilder: (_) => [
                  if (!revoked) const PopupMenuItem(value: 'rotate', child: Text('Rotate')),
                  if (!revoked) const PopupMenuItem(value: 'revoke', child: Text('Revoke')),
                  if (revoked) const PopupMenuItem(value: 'revoked', enabled: false, child: Text('Revoked')),
                ],
              ),
            ),
          );
        },
      )),
    );
  }
}
