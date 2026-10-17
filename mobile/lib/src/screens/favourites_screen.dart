import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:cached_network_image/cached_network_image.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';

class FavouritesScreen extends ConsumerStatefulWidget {
  const FavouritesScreen({super.key});
  @override
  ConsumerState<FavouritesScreen> createState() => _FavouritesScreenState();
}

class _FavouritesScreenState extends ConsumerState<FavouritesScreen> {
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
      final api = ref.read(apiClientProvider);
      final r = await api.dio.get('/api/favourites');
      final list = (r.data as List).map((e) => Post.fromJson(e as Map<String, dynamic>)).toList();
      setState(() { posts = list; loading = false; });
    } catch (e) {
      setState(() { loading = false; error = e.toString(); });
    }
  }

  @override
  Widget build(BuildContext context) {
    final api = ref.watch(apiClientProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Favourites')),
      body: loading
          ? const Center(child: CircularProgressIndicator())
          : error != null
              ? Center(child: Column(mainAxisSize: MainAxisSize.min, children: [Text(error!), const SizedBox(height: 12), FilledButton(onPressed: _load, child: const Text('Retry'))]))
              : posts.isEmpty
                  ? Center(child: Column(mainAxisSize: MainAxisSize.min, children: [const Icon(Icons.favorite_border, size: 48), const SizedBox(height: 12), const Text('No favourites yet'), const SizedBox(height: 8), FilledButton(onPressed: () => context.go('/'), child: const Text('Browse feed'))]))
                  : RefreshIndicator(
                      onRefresh: _load,
                      child: ListView.separated(
                        padding: const EdgeInsets.all(12),
                        itemCount: posts.length,
                        separatorBuilder: (_, __) => const SizedBox(height: 8),
                        itemBuilder: (_, i) {
                          final p = posts[i];
                          final url = api.resolveMedia(p.mediaUrl);
                          return Card(
                            clipBehavior: Clip.antiAlias,
                            child: ListTile(
                              leading: url.isEmpty
                                  ? const Icon(Icons.lock)
                                  : ClipRRect(borderRadius: BorderRadius.circular(8), child: CachedNetworkImage(imageUrl: url, width: 56, height: 56, fit: BoxFit.cover)),
                              title: Text(p.title, maxLines: 1, overflow: TextOverflow.ellipsis),
                              subtitle: Text(p.description, maxLines: 1, overflow: TextOverflow.ellipsis),
                              trailing: const Icon(Icons.chevron_right),
                              onTap: () => context.push('/post/${p.id}'),
                            ),
                          );
                        },
                      ),
                    ),
    );
  }
}
