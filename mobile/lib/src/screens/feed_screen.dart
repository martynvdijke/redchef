import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';
import '../widgets/post_card.dart';

final tagsProvider = FutureProvider<List<TagWithCount>>((ref) async {
  final api = ref.watch(apiClientProvider);
  if (api.baseUrl == null) return [];
  final r = await api.dio.get('/api/tags');
  final list = r.data as List;
  return list.map((e) => TagWithCount.fromJson(e as Map<String, dynamic>)).toList();
});

class FeedFilter {
  final String? tag;
  final String? q;
  const FeedFilter({this.tag, this.q});
}

final feedFilterProvider = StateProvider<FeedFilter>((_) => const FeedFilter());
final searchQueryProvider = StateProvider<String>((_) => '');

class FeedScreen extends ConsumerStatefulWidget {
  const FeedScreen({super.key});
  @override
  ConsumerState<FeedScreen> createState() => _FeedScreenState();
}

class _FeedScreenState extends ConsumerState<FeedScreen> {
  final scrollCtrl = ScrollController();
  final searchCtrl = TextEditingController();
  List<Post> posts = [];
  bool loading = true;
  bool loadingMore = false;
  String? error;
  int totalCount = 0;
  static const pageSize = 20;

  @override
  void initState() {
    super.initState();
    scrollCtrl.addListener(_onScroll);
    WidgetsBinding.instance.addPostFrameCallback((_) => _load(reset: true));
  }

  @override
  void dispose() {
    scrollCtrl.dispose();
    searchCtrl.dispose();
    super.dispose();
  }

  void _onScroll() {
    if (scrollCtrl.position.pixels > scrollCtrl.position.maxScrollExtent - 600) {
      if (!loadingMore && posts.length < totalCount) _load();
    }
  }

  Future<void> _load({bool reset = false}) async {
    final api = ref.read(apiClientProvider);
    if (api.baseUrl == null) {
      setState(() { error = 'No server configured'; loading = false; });
      return;
    }
    final filter = ref.read(feedFilterProvider);
    final offset = reset ? 0 : posts.length;
    if (reset) {
      setState(() { loading = true; error = null; });
    } else {
      setState(() => loadingMore = true);
    }
    try {
      final r = await api.dio.get('/api/posts', queryParameters: {
        'limit': pageSize,
        'offset': offset,
        if (filter.tag != null && filter.tag!.isNotEmpty) 'tag': filter.tag,
        if (filter.q != null && filter.q!.isNotEmpty) 'q': filter.q,
      });
      final totalHeader = r.headers.map['x-total-count']?.firstOrNull ?? r.headers.map['X-Total-Count']?.firstOrNull;
      if (totalHeader != null) totalCount = int.tryParse(totalHeader) ?? 0;
      final list = (r.data as List).map((e) => Post.fromJson(e as Map<String, dynamic>)).toList();
      setState(() {
        if (reset) {
          posts = list;
        } else {
          posts.addAll(list);
        }
        loading = false;
        loadingMore = false;
        error = null;
      });
    } on DioException catch (e) {
      setState(() {
        loading = false;
        loadingMore = false;
        error = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      });
    } catch (e) {
      setState(() { loading = false; loadingMore = false; error = e.toString(); });
    }
  }

  Future<void> _toggleFav(Post p) async {
    final auth = ref.read(authProvider);
    if (!auth.authenticated) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Log in to favourite posts')));
      return;
    }
    final wasFav = p.favourited;
    final oldCount = p.favouriteCount;
    // optimistic
    setState(() {
      final idx = posts.indexWhere((e) => e.id == p.id);
      if (idx != -1) {
        posts[idx] = Post(
          id: p.id, title: p.title, description: p.description, mediaType: p.mediaType, filename: p.filename, thumbnail: p.thumbnail,
          locked: p.locked, processing: p.processing, createdAt: p.createdAt, recipe: p.recipe, tags: p.tags, status: p.status, scheduledAt: p.scheduledAt,
          mediaUrl: p.mediaUrl, unlocked: p.unlocked, favourited: !wasFav, favouriteCount: wasFav ? (oldCount > 0 ? oldCount - 1 : 0) : oldCount + 1, tipCount: p.tipCount, linkedPosts: p.linkedPosts,
        );
      }
    });
    try {
      final notifier = ref.read(authProvider.notifier);
      final data = wasFav ? await notifier.unfavouritePost(p.id) : await notifier.favouritePost(p.id);
      final favourited = data['favourited'] == true;
      final count = (data['favourite_count'] as num?)?.toInt() ?? p.favouriteCount;
      setState(() {
        final idx = posts.indexWhere((e) => e.id == p.id);
        if (idx != -1) {
          posts[idx] = Post(
            id: p.id, title: p.title, description: p.description, mediaType: p.mediaType, filename: p.filename, thumbnail: p.thumbnail,
            locked: p.locked, processing: p.processing, createdAt: p.createdAt, recipe: p.recipe, tags: p.tags, status: p.status, scheduledAt: p.scheduledAt,
            mediaUrl: p.mediaUrl, unlocked: p.unlocked, favourited: favourited, favouriteCount: count, tipCount: p.tipCount, linkedPosts: p.linkedPosts,
          );
        }
      });
    } catch (e) {
      // revert
      setState(() {
        final idx = posts.indexWhere((e) => e.id == p.id);
        if (idx != -1) {
          posts[idx] = Post(
            id: p.id, title: p.title, description: p.description, mediaType: p.mediaType, filename: p.filename, thumbnail: p.thumbnail,
            locked: p.locked, processing: p.processing, createdAt: p.createdAt, recipe: p.recipe, tags: p.tags, status: p.status, scheduledAt: p.scheduledAt,
            mediaUrl: p.mediaUrl, unlocked: p.unlocked, favourited: wasFav, favouriteCount: oldCount, tipCount: p.tipCount, linkedPosts: p.linkedPosts,
          );
        }
      });
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Favourite failed: $e')));
    }
  }

  void _applySearch(String v) {
    ref.read(feedFilterProvider.notifier).state = FeedFilter(tag: ref.read(feedFilterProvider).tag, q: v.trim().isEmpty ? null : v.trim());
    _load(reset: true);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final api = ref.watch(apiClientProvider);
    final auth = ref.watch(authProvider);
    final tagsAsync = ref.watch(tagsProvider);
    final filter = ref.watch(feedFilterProvider);

    return Scaffold(
      appBar: AppBar(
        title: Row(children: [
          Container(padding: const EdgeInsets.all(8), decoration: BoxDecoration(color: scheme.primary, borderRadius: BorderRadius.circular(10)), child: const Icon(Icons.restaurant, color: Colors.white, size: 18)),
          const SizedBox(width: 10),
          const Text('RedChef'),
        ]),
        actions: [
          IconButton(icon: const Icon(Icons.bookmark_outline), onPressed: auth.authenticated ? () => context.push('/favourites') : null, tooltip: 'Favourites'),
          if (auth.authenticated)
            IconButton(icon: const Icon(Icons.add_a_photo_outlined), onPressed: () => context.push('/composer'), tooltip: 'New post'),
          PopupMenuButton<String>(
            onSelected: (v) {
              if (v == 'logout') ref.read(authProvider.notifier).logout();
              if (v == 'login') context.go('/login');
              if (v == 'server') context.go('/onboarding');
              if (v == 'account') context.push('/account');
              if (v == 'admin') context.push('/admin/posts');
              if (v == 'tokens') context.push('/tokens');
              if (v == 'refresh') { ref.invalidate(tagsProvider); _load(reset: true); ref.read(authProvider.notifier).refreshMe(); }
            },
            itemBuilder: (_) => [
              if (auth.authenticated) PopupMenuItem(value: 'logout', child: Text('Log out (${auth.me?.email ?? ''})')),
              if (!auth.authenticated) const PopupMenuItem(value: 'login', child: Text('Log in')),
              const PopupMenuItem(value: 'server', child: Text('Change server')),
              if (auth.authenticated) const PopupMenuItem(value: 'account', child: Text('Account')),
              if (auth.isAdmin) const PopupMenuItem(value: 'admin', child: Text('Admin posts')),
              if (auth.authenticated) const PopupMenuItem(value: 'tokens', child: Text('API tokens')),
              const PopupMenuItem(value: 'refresh', child: Text('Refresh')),
            ],
          ),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: () async { ref.invalidate(tagsProvider); await _load(reset: true); },
        child: CustomScrollView(
          controller: scrollCtrl,
          slivers: [
            SliverToBoxAdapter(
              child: Padding(
                padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
                child: TextField(
                  controller: searchCtrl,
                  decoration: InputDecoration(
                    hintText: 'Search recipes, titles…',
                    prefixIcon: const Icon(Icons.search),
                    suffixIcon: searchCtrl.text.isNotEmpty ? IconButton(icon: const Icon(Icons.clear), onPressed: () { searchCtrl.clear(); _applySearch(''); }) : null,
                  ),
                  onSubmitted: _applySearch,
                  onChanged: (v) => setState(() {}),
                ),
              ),
            ),
            SliverToBoxAdapter(
              child: SizedBox(
                height: 48,
                child: tagsAsync.when(
                  data: (tags) {
                    if (tags.isEmpty) return const SizedBox.shrink();
                    return ListView.separated(
                      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                      scrollDirection: Axis.horizontal,
                      itemCount: tags.length + 1,
                      separatorBuilder: (_, __) => const SizedBox(width: 8),
                      itemBuilder: (ctx, i) {
                        if (i == 0) {
                          final selected = filter.tag == null;
                          return ChoiceChip(
                            label: const Text('All'),
                            selected: selected,
                            onSelected: (_) { ref.read(feedFilterProvider.notifier).state = FeedFilter(q: filter.q); _load(reset: true); },
                          );
                        }
                        final t = tags[i - 1];
                        final selected = filter.tag == t.name;
                        return ChoiceChip(
                          label: Text('#${t.name} • ${t.count}'),
                          selected: selected,
                          onSelected: (_) {
                            ref.read(feedFilterProvider.notifier).state = FeedFilter(tag: selected ? null : t.name, q: filter.q);
                            _load(reset: true);
                          },
                        );
                      },
                    );
                  },
                  loading: () => const Padding(padding: EdgeInsets.all(16), child: LinearProgressIndicator()),
                  error: (_, __) => const SizedBox.shrink(),
                ),
              ),
            ),
            if (loading)
              const SliverToBoxAdapter(child: Padding(padding: EdgeInsets.all(32), child: Center(child: CircularProgressIndicator())))
            else if (error != null)
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.all(24),
                  child: Column(children: [
                    Icon(Icons.cloud_off, size: 48, color: scheme.error),
                    const SizedBox(height: 12),
                    Text(error!, textAlign: TextAlign.center),
                    const SizedBox(height: 12),
                    FilledButton(onPressed: () => _load(reset: true), child: const Text('Retry')),
                  ]),
                ),
              )
            else if (posts.isEmpty)
              SliverToBoxAdapter(
                child: Padding(
                  padding: const EdgeInsets.all(32),
                  child: Column(children: [
                    Icon(Icons.no_food_outlined, size: 56, color: scheme.onSurfaceVariant),
                    const SizedBox(height: 12),
                    Text('No posts yet', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
                    const SizedBox(height: 6),
                    Text('Try a different search or tag, or check back later.', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant), textAlign: TextAlign.center),
                  ]),
                ),
              )
            else
              SliverPadding(
                padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
                sliver: SliverGrid(
                  gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                    crossAxisCount: 2,
                    childAspectRatio: 0.72,
                    crossAxisSpacing: 12,
                    mainAxisSpacing: 12,
                  ),
                  delegate: SliverChildBuilderDelegate(
                    (ctx, i) {
                      if (i == posts.length) {
                        return const Center(child: Padding(padding: EdgeInsets.all(16), child: CircularProgressIndicator()));
                      }
                      final p = posts[i];
                      final url = api.resolveMedia(p.mediaUrl);
                      return PostCard(post: p, imageUrl: url, onTap: () => context.push('/post/${p.id}'), onFavourite: () => _toggleFav(p));
                    },
                    childCount: posts.length + (loadingMore ? 1 : 0),
                  ),
                ),
              ),
            const SliverToBoxAdapter(child: SizedBox(height: 24)),
          ],
        ),
      ),
      floatingActionButton: auth.authenticated
          ? FloatingActionButton.extended(onPressed: () => context.push('/composer'), icon: const Icon(Icons.add), label: const Text('New post'))
          : null,
    );
  }
}

extension _FirstOrNull<T> on List<T> {
  T? get firstOrNull => isEmpty ? null : first;
}
