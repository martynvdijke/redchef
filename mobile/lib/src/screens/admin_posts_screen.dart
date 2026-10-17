import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';

class AdminPostsScreen extends ConsumerStatefulWidget {
  const AdminPostsScreen({super.key});
  @override
  ConsumerState<AdminPostsScreen> createState() => _AdminPostsScreenState();
}

class _AdminPostsScreenState extends ConsumerState<AdminPostsScreen> {
  String filter = 'draft';
  List<Post> posts = [];
  bool loading = true;
  String? error;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { loading = true; error = null; });
    try {
      final list = await ref.read(authProvider.notifier).fetchAdminPosts(status: filter);
      setState(() { posts = list; loading = false; });
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      setState(() { loading = false; error = msg ?? 'Failed to load'; });
    } catch (e) {
      setState(() { loading = false; error = e.toString(); });
    }
  }

  Future<void> _delete(Post p) async {
    final confirm = await showDialog<bool>(context: context, builder: (ctx) => AlertDialog(
      title: const Text('Delete post?'),
      content: Text('Delete "${p.title}"? This cannot be undone.'),
      actions: [TextButton(onPressed: () => Navigator.pop(ctx, false), child: const Text('Cancel')), FilledButton(onPressed: () => Navigator.pop(ctx, true), child: const Text('Delete'))],
    ));
    if (confirm != true) return;
    try {
      await ref.read(authProvider.notifier).deleteAdminPost(p.id);
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Deleted')));
      _load();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Delete failed')));
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final auth = ref.watch(authProvider);
    if (!auth.isAdmin) {
      return Scaffold(
        appBar: AppBar(title: const Text('Admin posts')),
        body: Center(child: Padding(padding: const EdgeInsets.all(24), child: Column(mainAxisSize: MainAxisSize.min, children: [Icon(Icons.admin_panel_settings_outlined, size: 56, color: scheme.error), const SizedBox(height: 12), const Text('Admin only', style: TextStyle(fontWeight: FontWeight.w800, fontSize: 18)), const SizedBox(height: 8), Text('You need role == admin. Yours: ${auth.me?.role ?? 'none'}')]))),
      );
    }
    return Scaffold(
      appBar: AppBar(title: const Text('Admin posts')),
      body: Column(children: [
        Padding(
          padding: const EdgeInsets.all(12),
          child: SegmentedButton<String>(
            segments: const [ButtonSegment(value: 'draft', label: Text('Drafts'), icon: Icon(Icons.edit_note)), ButtonSegment(value: 'published', label: Text('Published'), icon: Icon(Icons.public))],
            selected: {filter},
            onSelectionChanged: (s) { setState(() => filter = s.first); _load(); },
          ),
        ),
        if (loading) const Expanded(child: Center(child: CircularProgressIndicator()))
        else if (error != null) Expanded(child: Center(child: Column(mainAxisSize: MainAxisSize.min, children: [Text(error!), const SizedBox(height: 12), FilledButton(onPressed: _load, child: const Text('Retry'))])))
        else if (posts.isEmpty) Expanded(child: Center(child: Column(mainAxisSize: MainAxisSize.min, children: [Icon(Icons.inbox_outlined, size: 48, color: scheme.onSurfaceVariant), const SizedBox(height: 8), Text('No $filter posts')])))
        else Expanded(
          child: RefreshIndicator(onRefresh: _load, child: ListView.separated(
            padding: const EdgeInsets.all(12),
            itemCount: posts.length,
            separatorBuilder: (_, __) => const SizedBox(height: 8),
            itemBuilder: (_, i) {
              final p = posts[i];
              final api = ref.read(apiClientProvider);
              final url = api.resolveMedia(p.mediaUrl);
              return Card(child: ListTile(
                leading: url.isEmpty ? const Icon(Icons.image_not_supported) : null,
                title: Text(p.title, maxLines: 1, overflow: TextOverflow.ellipsis),
                subtitle: Text('${p.status} • ${p.tags.join(', ')}', maxLines: 1, overflow: TextOverflow.ellipsis),
                trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                  IconButton(icon: const Icon(Icons.edit_outlined), onPressed: () => context.push('/composer', extra: p)),
                  IconButton(icon: const Icon(Icons.delete_outline, color: Colors.red), onPressed: () => _delete(p)),
                ]),
                onTap: () => context.push('/post/${p.id}'),
              ));
            },
          )),
        ),
      ]),
    );
  }
}
