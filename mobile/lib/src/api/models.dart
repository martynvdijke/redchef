class RecipeData {
  final List<String> ingredients;
  final List<String> steps;
  final int servings;
  final int prepMinutes;
  final int cookMinutes;

  const RecipeData({
    this.ingredients = const [],
    this.steps = const [],
    this.servings = 0,
    this.prepMinutes = 0,
    this.cookMinutes = 0,
  });

  factory RecipeData.fromJson(Map<String, dynamic>? json) {
    if (json == null) return const RecipeData();
    return RecipeData(
      ingredients: (json['ingredients'] as List?)?.map((e) => e.toString()).toList() ?? [],
      steps: (json['steps'] as List?)?.map((e) => e.toString()).toList() ?? [],
      servings: (json['servings'] as num?)?.toInt() ?? 0,
      prepMinutes: (json['prep_minutes'] as num?)?.toInt() ?? 0,
      cookMinutes: (json['cook_minutes'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
        'ingredients': ingredients,
        'steps': steps,
        'servings': servings,
        'prep_minutes': prepMinutes,
        'cook_minutes': cookMinutes,
      };

  bool get isEmpty => ingredients.isEmpty && steps.isEmpty && servings == 0 && prepMinutes == 0 && cookMinutes == 0;
}

class PostLink {
  final int id;
  final int postId;
  final int linkedPostId;
  final String linkedTitle;
  final int orderIndex;

  PostLink({required this.id, required this.postId, required this.linkedPostId, required this.linkedTitle, required this.orderIndex});

  factory PostLink.fromJson(Map<String, dynamic> j) => PostLink(
        id: (j['id'] as num).toInt(),
        postId: (j['post_id'] as num).toInt(),
        linkedPostId: (j['linked_post_id'] as num).toInt(),
        linkedTitle: j['linked_title']?.toString() ?? '',
        orderIndex: (j['order_index'] as num?)?.toInt() ?? 0,
      );
}

class Post {
  final int id;
  final String title;
  final String description;
  final String mediaType; // photo | video
  final String filename;
  final String thumbnail;
  final bool locked;
  final bool processing;
  final DateTime createdAt;
  final RecipeData recipe;
  final List<String> tags;
  final String status;
  final DateTime? scheduledAt;
  final String? mediaUrl; // relative like /uploads/...
  final bool unlocked;
  final bool favourited;
  final int favouriteCount;
  final int tipCount;
  final List<PostLink> linkedPosts;

  Post({
    required this.id,
    required this.title,
    required this.description,
    required this.mediaType,
    required this.filename,
    required this.thumbnail,
    required this.locked,
    required this.processing,
    required this.createdAt,
    required this.recipe,
    required this.tags,
    required this.status,
    this.scheduledAt,
    this.mediaUrl,
    required this.unlocked,
    required this.favourited,
    required this.favouriteCount,
    required this.tipCount,
    required this.linkedPosts,
  });

  factory Post.fromJson(Map<String, dynamic> j) {
    DateTime parseTime(dynamic v) {
      if (v == null) return DateTime.now();
      try {
        return DateTime.parse(v.toString());
      } catch (_) {
        return DateTime.now();
      }
    }

    return Post(
      id: (j['id'] as num).toInt(),
      title: j['title']?.toString() ?? '',
      description: j['description']?.toString() ?? '',
      mediaType: j['media_type']?.toString() ?? 'photo',
      filename: j['filename']?.toString() ?? '',
      thumbnail: j['thumbnail']?.toString() ?? '',
      locked: j['locked'] == true || j['locked'] == 1,
      processing: j['processing'] == true || j['processing'] == 1,
      createdAt: parseTime(j['created_at']),
      recipe: RecipeData.fromJson((j['recipe'] as Map?)?.cast<String, dynamic>()),
      tags: (j['tags'] as List?)?.map((e) => e.toString()).toList() ?? [],
      status: j['status']?.toString() ?? 'published',
      scheduledAt: j['scheduled_at'] != null ? DateTime.tryParse(j['scheduled_at'].toString()) : null,
      mediaUrl: j['media_url']?.toString(),
      unlocked: j['unlocked'] == true,
      favourited: j['favourited'] == true,
      favouriteCount: (j['favourite_count'] as num?)?.toInt() ?? 0,
      tipCount: (j['tip_count'] as num?)?.toInt() ?? 0,
      linkedPosts: (j['linked_posts'] as List?)?.map((e) => PostLink.fromJson(e as Map<String, dynamic>)).toList() ?? [],
    );
  }
}

class TagWithCount {
  final String name;
  final int count;
  TagWithCount({required this.name, required this.count});
  factory TagWithCount.fromJson(Map<String, dynamic> j) => TagWithCount(
        name: j['name'].toString(),
        count: (j['count'] as num).toInt(),
      );
}

class Comment {
  final int id;
  final int postId;
  final int userId;
  final String username;
  final int? parentId;
  final String body;
  final DateTime createdAt;
  Comment({required this.id, required this.postId, required this.userId, required this.username, this.parentId, required this.body, required this.createdAt});
  factory Comment.fromJson(Map<String, dynamic> j) => Comment(
        id: (j['id'] as num).toInt(),
        postId: (j['post_id'] as num).toInt(),
        userId: (j['user_id'] as num).toInt(),
        username: j['username']?.toString() ?? j['email']?.toString() ?? '',
        parentId: (j['parent_id'] as num?)?.toInt(),
        body: j['body']?.toString() ?? '',
        createdAt: DateTime.tryParse(j['created_at']?.toString() ?? '') ?? DateTime.now(),
      );
}

class MeResponse {
  final bool authenticated;
  final int? id;
  final String? email;
  final String? role;
  final bool? paid;
  MeResponse({required this.authenticated, this.id, this.email, this.role, this.paid});
  factory MeResponse.fromJson(Map<String, dynamic> j) => MeResponse(
        authenticated: j['authenticated'] == true,
        id: (j['id'] as num?)?.toInt(),
        email: j['email']?.toString(),
        role: j['role']?.toString(),
        paid: j['paid'] == true,
      );
  bool get isAdmin => role == 'admin';
}

class TipResult {
  final bool ok;
  final int tipCount;
  final int amountCents;
  final int totalAmount;
  final String formatted;
  TipResult({required this.ok, required this.tipCount, required this.amountCents, required this.totalAmount, required this.formatted});
  factory TipResult.fromJson(Map<String, dynamic> j) => TipResult(
        ok: j['ok'] == true || j['ok'] == 1,
        tipCount: (j['tip_count'] as num?)?.toInt() ?? 0,
        amountCents: (j['amount_cents'] as num?)?.toInt() ?? 0,
        totalAmount: (j['total_amount'] as num?)?.toInt() ?? 0,
        formatted: j['formatted']?.toString() ?? '',
      );
}

class SetupStatus {
  final bool isRedChef;
  final Map<String, dynamic> raw;
  SetupStatus({required this.isRedChef, required this.raw});
  factory SetupStatus.fromJson(Map<String, dynamic> j) => SetupStatus(isRedChef: true, raw: j);
}

class ApiToken {
  final int id;
  final String name;
  final String? token; // only on creation/rotate
  final DateTime? createdAt;
  final DateTime? lastUsedAt;
  final DateTime? expiresAt;
  final DateTime? revokedAt;
  ApiToken({required this.id, required this.name, this.token, this.createdAt, this.lastUsedAt, this.expiresAt, this.revokedAt});
  factory ApiToken.fromJson(Map<String, dynamic> j) => ApiToken(
        id: (j['id'] as num).toInt(),
        name: j['name']?.toString() ?? '',
        token: j['token']?.toString(),
        createdAt: j['created_at'] != null ? DateTime.tryParse(j['created_at'].toString()) : null,
        lastUsedAt: j['last_used_at'] != null ? DateTime.tryParse(j['last_used_at'].toString()) : null,
        expiresAt: j['expires_at'] != null ? DateTime.tryParse(j['expires_at'].toString()) : null,
        revokedAt: j['revoked_at'] != null ? DateTime.tryParse(j['revoked_at'].toString()) : null,
      );
  bool get isRevoked => revokedAt != null;
}

class CreatePostResponse {
  final int id;
  final String title;
  final String mediaType;
  final bool locked;
  final DateTime? createdAt;
  final bool processing;
  final String message;
  CreatePostResponse({required this.id, required this.title, required this.mediaType, required this.locked, this.createdAt, required this.processing, required this.message});
  factory CreatePostResponse.fromJson(Map<String, dynamic> j) => CreatePostResponse(
        id: (j['id'] as num).toInt(),
        title: j['title']?.toString() ?? '',
        mediaType: j['media_type']?.toString() ?? 'photo',
        locked: j['locked'] == true || j['locked'] == 1 || j['locked'] == 'true',
        createdAt: j['created_at'] != null ? DateTime.tryParse(j['created_at'].toString()) : null,
        processing: j['processing'] == true || j['processing'] == 1,
        message: j['message']?.toString() ?? '',
      );
}
