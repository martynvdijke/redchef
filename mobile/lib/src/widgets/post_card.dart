import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import '../api/models.dart';

class PostCard extends StatelessWidget {
  final Post post;
  final String imageUrl;
  final VoidCallback onTap;
  final VoidCallback onFavourite;
  const PostCard({super.key, required this.post, required this.imageUrl, required this.onTap, required this.onFavourite});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: onTap,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            AspectRatio(
              aspectRatio: post.mediaType == 'video' ? 16 / 10 : 4 / 3,
              child: Stack(
                fit: StackFit.expand,
                children: [
                  if (imageUrl.isEmpty)
                    Container(
                      color: scheme.surfaceContainerHighest,
                      child: Icon(Icons.lock_outline, size: 36, color: scheme.onSurfaceVariant),
                    )
                  else
                    CachedNetworkImage(
                      imageUrl: imageUrl,
                      fit: BoxFit.cover,
                      placeholder: (_, __) => Container(color: scheme.surfaceContainerHighest),
                      errorWidget: (_, __, ___) => Container(
                        color: scheme.surfaceContainerHighest,
                        child: const Icon(Icons.broken_image_outlined),
                      ),
                    ),
                  if (post.locked && !post.unlocked)
                    Positioned.fill(
                      child: Container(
                        decoration: BoxDecoration(
                          gradient: LinearGradient(
                            begin: Alignment.topCenter,
                            end: Alignment.bottomCenter,
                            colors: [Colors.transparent, Colors.black.withValues(alpha: 0.55)],
                          ),
                        ),
                      ),
                    ),
                  if (post.locked && !post.unlocked)
                    const Positioned(
                      top: 10,
                      right: 10,
                      child: _Pill(icon: Icons.lock, label: 'Locked'),
                    ),
                  if (post.mediaType == 'video' && post.unlocked)
                    const Positioned(
                      top: 10,
                      left: 10,
                      child: _Pill(icon: Icons.play_arrow, label: 'Video'),
                    ),
                  Positioned(
                    bottom: 8,
                    right: 8,
                    child: Material(
                      color: Colors.black.withValues(alpha: 0.55),
                      borderRadius: BorderRadius.circular(20),
                      child: InkWell(
                        onTap: onFavourite,
                        borderRadius: BorderRadius.circular(20),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(post.favourited ? Icons.favorite : Icons.favorite_border,
                                  size: 16, color: post.favourited ? Colors.redAccent : Colors.white),
                              const SizedBox(width: 4),
                              Text('${post.favouriteCount}',
                                  style: const TextStyle(color: Colors.white, fontSize: 12, fontWeight: FontWeight.w700)),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            Padding(
              padding: const EdgeInsets.fromLTRB(14, 12, 14, 6),
              child: Text(
                post.title,
                maxLines: 2,
                overflow: TextOverflow.ellipsis,
                style: Theme.of(context).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800, height: 1.2),
              ),
            ),
            if (post.description.isNotEmpty)
              Padding(
                padding: const EdgeInsets.symmetric(horizontal: 14),
                child: Text(
                  post.description,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant),
                ),
              ),
            if (post.tags.isNotEmpty)
              Padding(
                padding: const EdgeInsets.fromLTRB(14, 8, 14, 12),
                child: Wrap(
                  spacing: 6,
                  runSpacing: 4,
                  children: post.tags.take(3).map((t) => Container(
                    padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
                    decoration: BoxDecoration(
                      color: scheme.secondaryContainer,
                      borderRadius: BorderRadius.circular(20),
                    ),
                    child: Text('#$t', style: TextStyle(fontSize: 11, fontWeight: FontWeight.w600, color: scheme.onSecondaryContainer)),
                  )).toList(),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _Pill extends StatelessWidget {
  final IconData icon;
  final String label;
  const _Pill({required this.icon, required this.label});
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 5),
      decoration: BoxDecoration(color: Colors.black.withValues(alpha: 0.65), borderRadius: BorderRadius.circular(20)),
      child: Row(mainAxisSize: MainAxisSize.min, children: [
        Icon(icon, size: 12, color: Colors.white),
        const SizedBox(width: 4),
        Text(label, style: const TextStyle(color: Colors.white, fontSize: 11, fontWeight: FontWeight.w700)),
      ]),
    );
  }
}
