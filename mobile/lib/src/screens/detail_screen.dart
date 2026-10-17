import 'package:cached_network_image/cached_network_image.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:intl/intl.dart';
import '../api/models.dart';
import '../auth/auth_provider.dart';

class DetailScreen extends ConsumerStatefulWidget {
  final int postId;
  const DetailScreen({super.key, required this.postId});
  @override
  ConsumerState<DetailScreen> createState() => _DetailScreenState();
}

class _DetailScreenState extends ConsumerState<DetailScreen> {
  Post? post;
  bool loading = true;
  String? error;
  List<Comment> comments = [];
  bool commentsLoading = true;
  final commentCtrl = TextEditingController();
  bool postingComment = false;
  bool unlocking = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() { loading = true; error = null; });
    try {
      final api = ref.read(apiClientProvider);
      final r = await api.dio.get('/api/posts/${widget.postId}');
      final p = Post.fromJson(r.data as Map<String, dynamic>);
      setState(() { post = p; loading = false; });
      _loadComments();
    } on DioException catch (e) {
      setState(() { loading = false; error = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message; });
    } catch (e) {
      setState(() { loading = false; error = e.toString(); });
    }
  }

  Future<void> _loadComments() async {
    setState(() => commentsLoading = true);
    try {
      final api = ref.read(apiClientProvider);
      final r = await api.dio.get('/api/posts/${widget.postId}/comments');
      final list = (r.data as List).map((e) => Comment.fromJson(e as Map<String, dynamic>)).toList();
      setState(() { comments = list; commentsLoading = false; });
    } catch (_) {
      setState(() => commentsLoading = false);
    }
  }

  Future<void> _toggleFav() async {
    final auth = ref.read(authProvider);
    if (!auth.authenticated) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Log in to favourite')));
      return;
    }
    final wasFav = post?.favourited ?? false;
    final oldCount = post?.favouriteCount ?? 0;
    // optimistic
    setState(() {
      if (post != null) {
        post = Post(
          id: post!.id, title: post!.title, description: post!.description, mediaType: post!.mediaType, filename: post!.filename, thumbnail: post!.thumbnail,
          locked: post!.locked, processing: post!.processing, createdAt: post!.createdAt, recipe: post!.recipe, tags: post!.tags, status: post!.status, scheduledAt: post!.scheduledAt,
          mediaUrl: post!.mediaUrl, unlocked: post!.unlocked, favourited: !wasFav, favouriteCount: wasFav ? (oldCount > 0 ? oldCount - 1 : 0) : oldCount + 1, tipCount: post!.tipCount, linkedPosts: post!.linkedPosts,
        );
      }
    });
    try {
      final notifier = ref.read(authProvider.notifier);
      final data = wasFav ? await notifier.unfavouritePost(widget.postId) : await notifier.favouritePost(widget.postId);
      setState(() {
        if (post != null) {
          post = Post(
            id: post!.id, title: post!.title, description: post!.description, mediaType: post!.mediaType, filename: post!.filename, thumbnail: post!.thumbnail,
            locked: post!.locked, processing: post!.processing, createdAt: post!.createdAt, recipe: post!.recipe, tags: post!.tags, status: post!.status, scheduledAt: post!.scheduledAt,
            mediaUrl: post!.mediaUrl, unlocked: post!.unlocked, favourited: data['favourited'] == true, favouriteCount: (data['favourite_count'] as num?)?.toInt() ?? post!.favouriteCount, tipCount: post!.tipCount, linkedPosts: post!.linkedPosts,
          );
        }
      });
    } catch (e) {
      // revert optimistic
      setState(() {
        if (post != null) {
          post = Post(
            id: post!.id, title: post!.title, description: post!.description, mediaType: post!.mediaType, filename: post!.filename, thumbnail: post!.thumbnail,
            locked: post!.locked, processing: post!.processing, createdAt: post!.createdAt, recipe: post!.recipe, tags: post!.tags, status: post!.status, scheduledAt: post!.scheduledAt,
            mediaUrl: post!.mediaUrl, unlocked: post!.unlocked, favourited: wasFav, favouriteCount: oldCount, tipCount: post!.tipCount, linkedPosts: post!.linkedPosts,
          );
        }
      });
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Failed: $e')));
    }
  }

  Future<void> _tip(int cents) async {
    final auth = ref.read(authProvider);
    if (!auth.authenticated) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Log in to tip')));
      return;
    }
    try {
      final res = await ref.read(authProvider.notifier).tipPost(widget.postId, cents);
      setState(() {
        if (post != null) {
          post = Post(
            id: post!.id, title: post!.title, description: post!.description, mediaType: post!.mediaType, filename: post!.filename, thumbnail: post!.thumbnail,
            locked: post!.locked, processing: post!.processing, createdAt: post!.createdAt, recipe: post!.recipe, tags: post!.tags, status: post!.status, scheduledAt: post!.scheduledAt,
            mediaUrl: post!.mediaUrl, unlocked: post!.unlocked, favourited: post!.favourited, favouriteCount: post!.favouriteCount, tipCount: res.tipCount, linkedPosts: post!.linkedPosts,
          );
        }
      });
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Tipped ${res.formatted.isEmpty ? '${cents / 100}€' : res.formatted}! Total tips: ${res.tipCount}')));
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? (e.response!.data['error']?.toString() ?? e.response!.data['message']?.toString()) : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Tip failed')));
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text('Tip failed: $e')));
    }
  }

  Future<void> _postComment() async {
    final text = commentCtrl.text.trim();
    if (text.isEmpty) return;
    setState(() => postingComment = true);
    try {
      final api = ref.read(apiClientProvider);
      await api.dio.post('/api/posts/${widget.postId}/comments', data: {'body': text});
      commentCtrl.clear();
      await _loadComments();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Failed to post')));
    } finally {
      if (mounted) setState(() => postingComment = false);
    }
  }

  Future<void> _unlockItem() async {
    setState(() => unlocking = true);
    try {
      final api = ref.read(apiClientProvider);
      await api.dio.post('/api/pay/item', data: {'post_id': widget.postId});
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Unlocked! Refreshing…')));
      await _load();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Unlock failed')));
    } finally {
      if (mounted) setState(() => unlocking = false);
    }
  }

  Future<void> _unlockSubscription() async {
    setState(() => unlocking = true);
    try {
      final api = ref.read(apiClientProvider);
      await api.dio.post('/api/pay/unlock');
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Subscribed! All content unlocked.')));
      await _load();
      await ref.read(authProvider.notifier).refreshMe();
    } on DioException catch (e) {
      final msg = e.response?.data is Map ? e.response!.data['error']?.toString() : e.message;
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(msg ?? 'Subscription failed')));
    } finally {
      if (mounted) setState(() => unlocking = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final api = ref.watch(apiClientProvider);
    final auth = ref.watch(authProvider);

    if (loading) {
      return Scaffold(appBar: AppBar(), body: const Center(child: CircularProgressIndicator()));
    }
    if (error != null || post == null) {
      return Scaffold(
        appBar: AppBar(),
        body: Center(child: Padding(padding: const EdgeInsets.all(24), child: Column(mainAxisSize: MainAxisSize.min, children: [
          Icon(Icons.error_outline, size: 48, color: scheme.error),
          const SizedBox(height: 12),
          Text(error ?? 'Not found', textAlign: TextAlign.center),
          const SizedBox(height: 12),
          FilledButton(onPressed: _load, child: const Text('Retry')),
        ]))),
      );
    }
    final p = post!;
    final imageUrl = api.resolveMedia(p.mediaUrl);
    final isLocked = p.locked && !p.unlocked;

    return Scaffold(
      appBar: AppBar(
        title: Text(p.title, overflow: TextOverflow.ellipsis),
        actions: [
          IconButton(icon: Icon(p.favourited ? Icons.favorite : Icons.favorite_border, color: p.favourited ? Colors.redAccent : null), onPressed: _toggleFav),
          if (auth.isAdmin) IconButton(icon: const Icon(Icons.edit_outlined), onPressed: () => context.push('/composer', extra: p)),
        ],
      ),
      body: RefreshIndicator(
        onRefresh: _load,
        child: SingleChildScrollView(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              AspectRatio(
                aspectRatio: 4 / 3,
                child: isLocked
                    ? Container(
                        color: scheme.surfaceContainerHighest,
                        child: Column(mainAxisAlignment: MainAxisAlignment.center, children: [
                          Icon(Icons.lock, size: 48, color: scheme.onSurfaceVariant),
                          const SizedBox(height: 8),
                          Text('Locked content', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w700)),
                          const SizedBox(height: 4),
                          Text('Subscribe or unlock this item to view', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
                          const SizedBox(height: 16),
                          Wrap(spacing: 8, children: [
                            FilledButton(onPressed: unlocking ? null : _unlockItem, child: Text(unlocking ? '…' : 'Unlock item')),
                            OutlinedButton(onPressed: unlocking ? null : _unlockSubscription, child: Text(unlocking ? '…' : 'Subscribe €4,99/mo')),
                          ]),
                        ]),
                      )
                    : imageUrl.isEmpty
                        ? Container(color: scheme.surfaceContainerHighest, child: const Icon(Icons.image_not_supported_outlined, size: 48))
                        : CachedNetworkImage(imageUrl: imageUrl, fit: BoxFit.cover, width: double.infinity, placeholder: (_, __) => Container(color: scheme.surfaceContainerHighest), errorWidget: (_, __, ___) => const Icon(Icons.broken_image)),
              ),
              Padding(
                padding: const EdgeInsets.all(16),
                child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Row(children: [
                    Icon(Icons.access_time, size: 14, color: scheme.onSurfaceVariant),
                    const SizedBox(width: 4),
                    Text(DateFormat.yMMMd().format(p.createdAt), style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
                    const Spacer(),
                    Icon(p.favourited ? Icons.favorite : Icons.favorite_border, size: 16, color: p.favourited ? Colors.redAccent : scheme.onSurfaceVariant),
                    const SizedBox(width: 4),
                    Text('${p.favouriteCount}', style: Theme.of(context).textTheme.bodySmall),
                    const SizedBox(width: 12),
                    const Icon(Icons.chat_bubble_outline, size: 16),
                    const SizedBox(width: 4),
                    Text('${comments.length}', style: Theme.of(context).textTheme.bodySmall),
                  ]),
                  const SizedBox(height: 12),
                  Text(p.title, style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.w900, height: 1.1)),
                  if (p.description.isNotEmpty) ...[
                    const SizedBox(height: 8),
                    Text(p.description, style: Theme.of(context).textTheme.bodyMedium?.copyWith(color: scheme.onSurfaceVariant, height: 1.5)),
                  ],
                  if (p.tags.isNotEmpty) ...[
                    const SizedBox(height: 12),
                    Wrap(spacing: 8, runSpacing: 6, children: p.tags.map((t) => Chip(label: Text('#$t'), backgroundColor: scheme.secondaryContainer, labelStyle: TextStyle(color: scheme.onSecondaryContainer, fontWeight: FontWeight.w600, fontSize: 13))).toList()),
                  ],
                  if (!isLocked && !p.recipe.isEmpty) ...[
                    const SizedBox(height: 20),
                    _RecipeCard(recipe: p.recipe),
                  ],
                  const SizedBox(height: 16),
                  Container(
                    padding: const EdgeInsets.all(14),
                    decoration: BoxDecoration(
                      color: scheme.secondaryContainer.withValues(alpha: 0.6),
                      borderRadius: BorderRadius.circular(16),
                      border: Border.all(color: scheme.outlineVariant),
                    ),
                    child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                      Row(children: [
                        Icon(Icons.volunteer_activism_outlined, color: scheme.primary, size: 20),
                        const SizedBox(width: 8),
                        Text('Tip the chef', style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w800)),
                        const Spacer(),
                        Container(
                          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                          decoration: BoxDecoration(color: scheme.primaryContainer, borderRadius: BorderRadius.circular(20)),
                          child: Row(mainAxisSize: MainAxisSize.min, children: [
                            Icon(Icons.favorite, size: 12, color: scheme.onPrimaryContainer),
                            const SizedBox(width: 4),
                            Text('${p.tipCount} tips', style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, color: scheme.onPrimaryContainer)),
                          ]),
                        ),
                      ]),
                      const SizedBox(height: 10),
                      Wrap(spacing: 8, runSpacing: 8, children: [
                        _TipButton(label: '€1', onTap: () => _tip(100)),
                        _TipButton(label: '€3', onTap: () => _tip(300)),
                        _TipButton(label: '€5', onTap: () => _tip(500)),
                        _TipButton(label: '€10', onTap: () => _tip(1000)),
                      ]),
                      if (!auth.authenticated)
                        Padding(
                          padding: const EdgeInsets.only(top: 8),
                          child: Text('Log in to send a tip', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
                        ),
                    ]),
                  ),
                  if (p.linkedPosts.isNotEmpty) ...[
                    const SizedBox(height: 16),
                    Text('Related', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
                    const SizedBox(height: 8),
                    ...p.linkedPosts.map((l) => Card(
                          child: ListTile(
                            title: Text(l.linkedTitle.isEmpty ? 'Post #${l.linkedPostId}' : l.linkedTitle),
                            trailing: const Icon(Icons.chevron_right),
                            onTap: () => context.push('/post/${l.linkedPostId}'),
                          ),
                        )),
                  ],
                  const SizedBox(height: 24),
                  Row(children: [
                    Text('Comments', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
                    const SizedBox(width: 8),
                    Container(padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2), decoration: BoxDecoration(color: scheme.primaryContainer, borderRadius: BorderRadius.circular(20)), child: Text('${comments.length}', style: TextStyle(color: scheme.onPrimaryContainer, fontWeight: FontWeight.w700, fontSize: 12))),
                  ]),
                  const SizedBox(height: 12),
                  if (auth.authenticated)
                    Row(children: [
                      Expanded(
                        child: TextField(
                          controller: commentCtrl,
                          decoration: const InputDecoration(hintText: 'Add a comment…'),
                          minLines: 1,
                          maxLines: 3,
                        ),
                      ),
                      const SizedBox(width: 8),
                      FilledButton(
                        onPressed: postingComment ? null : _postComment,
                        child: postingComment ? const SizedBox(height: 16, width: 16, child: CircularProgressIndicator(strokeWidth: 2, color: Colors.white)) : const Text('Post'),
                      ),
                    ])
                  else
                    Container(
                      padding: const EdgeInsets.all(12),
                      decoration: BoxDecoration(color: scheme.surfaceContainerHighest, borderRadius: BorderRadius.circular(12)),
                      child: Row(children: [
                        const Icon(Icons.lock_outline, size: 18),
                        const SizedBox(width: 8),
                        const Expanded(child: Text('Log in to comment')),
                        TextButton(onPressed: () => context.go('/login'), child: const Text('Log in')),
                      ]),
                    ),
                  const SizedBox(height: 12),
                  if (commentsLoading)
                    const Padding(padding: EdgeInsets.all(16), child: Center(child: CircularProgressIndicator()))
                  else if (comments.isEmpty)
                    Padding(padding: const EdgeInsets.symmetric(vertical: 12), child: Text('No comments yet — be the first!', style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)))
                  else
                    ...comments.map((c) => Card(
                          margin: const EdgeInsets.only(bottom: 8),
                          child: Padding(
                            padding: const EdgeInsets.all(12),
                            child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                              Row(children: [
                                CircleAvatar(radius: 14, backgroundColor: scheme.primaryContainer, child: Text(c.username.isEmpty ? '?' : c.username[0].toUpperCase(), style: TextStyle(fontSize: 12, color: scheme.onPrimaryContainer, fontWeight: FontWeight.w700))),
                                const SizedBox(width: 8),
                                Expanded(child: Text(c.username, style: const TextStyle(fontWeight: FontWeight.w700, fontSize: 13))),
                                Text(DateFormat.MMMd().add_jm().format(c.createdAt), style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 11)),
                              ]),
                              const SizedBox(height: 6),
                              Text(c.body, style: Theme.of(context).textTheme.bodyMedium),
                            ]),
                          ),
                        )),
                  const SizedBox(height: 24),
                ]),
              ),
            ],
          ),
        ),
      ),
    );
  }
}

class _RecipeCard extends StatelessWidget {
  final RecipeData recipe;
  const _RecipeCard({required this.recipe});
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(color: scheme.surfaceContainerHighest.withValues(alpha: 0.6), borderRadius: BorderRadius.circular(16), border: Border.all(color: scheme.outlineVariant)),
      child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
        Row(children: [
          Icon(Icons.menu_book_outlined, color: scheme.primary),
          const SizedBox(width: 8),
          Text('Recipe', style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800)),
          const Spacer(),
          if (recipe.servings > 0)
            Chip(label: Text('${recipe.servings} servings'), visualDensity: VisualDensity.compact, backgroundColor: scheme.primaryContainer),
        ]),
        if (recipe.prepMinutes > 0 || recipe.cookMinutes > 0) ...[
          const SizedBox(height: 8),
          Wrap(spacing: 8, children: [
            if (recipe.prepMinutes > 0) _MetaChip(icon: Icons.timer_outlined, label: 'Prep ${recipe.prepMinutes}′'),
            if (recipe.cookMinutes > 0) _MetaChip(icon: Icons.local_fire_department_outlined, label: 'Cook ${recipe.cookMinutes}′'),
          ]),
        ],
        if (recipe.ingredients.isNotEmpty) ...[
          const SizedBox(height: 14),
          Text('Ingredients', style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(height: 6),
          ...recipe.ingredients.map((e) => Padding(padding: const EdgeInsets.symmetric(vertical: 2), child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [Text('• ', style: TextStyle(color: scheme.primary, fontWeight: FontWeight.w800)), Expanded(child: Text(e))]))),
        ],
        if (recipe.steps.isNotEmpty) ...[
          const SizedBox(height: 14),
          Text('Steps', style: Theme.of(context).textTheme.titleSmall?.copyWith(fontWeight: FontWeight.w700)),
          const SizedBox(height: 6),
          ...recipe.steps.asMap().entries.map((en) => Padding(
                padding: const EdgeInsets.symmetric(vertical: 4),
                child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Container(width: 24, height: 24, decoration: BoxDecoration(color: scheme.primary, shape: BoxShape.circle), child: Center(child: Text('${en.key + 1}', style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w800)))),
                  const SizedBox(width: 8),
                  Expanded(child: Text(en.value, style: Theme.of(context).textTheme.bodyMedium)),
                ]),
              )),
        ],
      ]),
    );
  }
}

class _MetaChip extends StatelessWidget {
  final IconData icon;
  final String label;
  const _MetaChip({required this.icon, required this.label});
  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
      decoration: BoxDecoration(color: scheme.secondaryContainer, borderRadius: BorderRadius.circular(20)),
      child: Row(mainAxisSize: MainAxisSize.min, children: [Icon(icon, size: 14, color: scheme.onSecondaryContainer), const SizedBox(width: 4), Text(label, style: TextStyle(fontSize: 12, fontWeight: FontWeight.w600, color: scheme.onSecondaryContainer))]),
    );
  }
}

class _TipButton extends StatelessWidget {
  final String label;
  final VoidCallback onTap;
  const _TipButton({required this.label, required this.onTap});
  @override
  Widget build(BuildContext context) {
    return FilledButton.tonal(
      onPressed: onTap,
      style: FilledButton.styleFrom(padding: const EdgeInsets.symmetric(horizontal: 14, vertical: 8), visualDensity: VisualDensity.compact),
      child: Text(label, style: const TextStyle(fontWeight: FontWeight.w800)),
    );
  }
}
