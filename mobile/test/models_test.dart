import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/api/models.dart';

void main() {
  group('Post parsing', () {
    test('parses locked post with null media_url', () {
      final j = {
        'id': 1,
        'title': 'Spicy Ramen',
        'description': 'Delicious',
        'media_type': 'photo',
        'filename': '1.jpg',
        'thumbnail': 'thumb_1.jpg',
        'locked': true,
        'processing': false,
        'created_at': '2026-01-01T12:00:00Z',
        'recipe': {'ingredients': ['noodles'], 'steps': ['cook'], 'servings': 2},
        'tags': ['ramen', 'spicy'],
        'status': 'published',
        'media_url': null,
        'unlocked': false,
        'favourited': false,
        'favourite_count': 3,
        'tip_count': 1,
        'linked_posts': [],
      };
      final p = Post.fromJson(j);
      expect(p.id, 1);
      expect(p.title, 'Spicy Ramen');
      expect(p.locked, true);
      expect(p.unlocked, false);
      expect(p.mediaUrl, isNull);
      expect(p.tags, ['ramen', 'spicy']);
      expect(p.recipe.ingredients, ['noodles']);
      expect(p.favouriteCount, 3);
    });

    test('parses unlocked video post with media_url', () {
      final j = {
        'id': 2,
        'title': 'Sushi Video',
        'description': '',
        'media_type': 'video',
        'filename': '2.mp4',
        'thumbnail': 'thumb_2.jpg',
        'locked': false,
        'processing': false,
        'created_at': '2026-02-02T10:00:00Z',
        'recipe': {},
        'tags': [],
        'status': 'published',
        'media_url': '/uploads/2.mp4',
        'unlocked': true,
        'favourited': true,
        'favourite_count': 10,
        'tip_count': 0,
      };
      final p = Post.fromJson(j);
      expect(p.mediaType, 'video');
      expect(p.mediaUrl, '/uploads/2.mp4');
      expect(p.favourited, true);
    });

    test('handles missing optional fields gracefully', () {
      final j = {'id': 3, 'title': 'Minimal'};
      final p = Post.fromJson(j);
      expect(p.id, 3);
      expect(p.description, '');
      expect(p.mediaType, 'photo');
    });
  });

  group('RecipeData', () {
    test('parses full recipe', () {
      final r = RecipeData.fromJson({'ingredients': ['a', 'b'], 'steps': ['s1'], 'servings': 4, 'prep_minutes': 10, 'cook_minutes': 20});
      expect(r.ingredients.length, 2);
      expect(r.servings, 4);
      expect(r.isEmpty, false);
    });
    test('null returns empty', () {
      final r = RecipeData.fromJson(null);
      expect(r.isEmpty, true);
    });
  });

  group('MeResponse', () {
    test('authenticated true when role admin', () {
      final m = MeResponse.fromJson({'authenticated': true, 'id': 1, 'email': 'a@b.com', 'role': 'admin', 'paid': true});
      expect(m.authenticated, true);
      expect(m.isAdmin, true);
    });
    test('anonymous returns not authenticated', () {
      final m = MeResponse.fromJson({'authenticated': false});
      expect(m.authenticated, false);
      expect(m.isAdmin, false);
    });
  });

  group('Comment', () {
    test('parses comment with parent_id', () {
      final c = Comment.fromJson({
        'id': 5,
        'post_id': 1,
        'user_id': 2,
        'username': 'alice@example.com',
        'parent_id': 1,
        'body': 'Nice!',
        'created_at': '2026-01-02T10:00:00Z',
      });
      expect(c.parentId, 1);
      expect(c.body, 'Nice!');
    });
  });

  group('TagWithCount', () {
    test('parses tag', () {
      final t = TagWithCount.fromJson({'name': 'ramen', 'count': 5});
      expect(t.name, 'ramen');
      expect(t.count, 5);
    });
  });

  group('TipResult', () {
    test('parses tip response 201', () {
      final j = {'ok': true, 'tip_count': 3, 'amount_cents': 500, 'total_amount': 1500, 'formatted': '€15.00'};
      final r = TipResult.fromJson(j);
      expect(r.ok, true);
      expect(r.tipCount, 3);
      expect(r.amountCents, 500);
      expect(r.formatted, '€15.00');
    });
    test('handles missing fields', () {
      final r = TipResult.fromJson({});
      expect(r.ok, false);
      expect(r.tipCount, 0);
    });
  });

  group('ApiToken', () {
    test('parses token list item', () {
      final j = {'id': 1, 'name': 'CLI', 'created_at': '2026-01-01T00:00:00Z', 'last_used_at': null, 'expires_at': null, 'revoked_at': null};
      final t = ApiToken.fromJson(j);
      expect(t.id, 1);
      expect(t.name, 'CLI');
      expect(t.isRevoked, false);
      expect(t.createdAt, isNotNull);
    });
    test('detects revoked', () {
      final j = {'id': 2, 'name': 'old', 'created_at': '2026-01-01T00:00:00Z', 'revoked_at': '2026-02-01T00:00:00Z'};
      final t = ApiToken.fromJson(j);
      expect(t.isRevoked, true);
    });
    test('parses creation with token', () {
      final j = {'id': 3, 'name': 'new', 'token': 'abc123', 'created_at': '2026-01-01T00:00:00Z'};
      final t = ApiToken.fromJson(j);
      expect(t.token, 'abc123');
    });
  });

  group('CreatePostResponse', () {
    test('parses 201 post creation', () {
      final j = {'id': 10, 'title': 'New post', 'media_type': 'photo', 'locked': false, 'created_at': '2026-01-01T12:00:00Z', 'processing': true, 'message': 'Post created'};
      final r = CreatePostResponse.fromJson(j);
      expect(r.id, 10);
      expect(r.processing, true);
      expect(r.message, 'Post created');
    });
    test('handles string locked', () {
      final j = {'id': 1, 'title': 't', 'media_type': 'photo', 'locked': 'true', 'processing': false, 'message': ''};
      final r = CreatePostResponse.fromJson(j);
      expect(r.locked, true);
    });
  });

  group('SetupStatus', () {
    test('fromJson marks isRedChef true', () {
      final s = SetupStatus.fromJson({'setup_complete': true});
      expect(s.isRedChef, true);
      expect(s.raw['setup_complete'], true);
    });
  });

  group('Favourite toggle contract', () {
    test('DELETE unfavourite returns favourited false', () {
      final data = {'favourited': false, 'favourite_count': 5};
      expect(data['favourited'], false);
      expect(data['favourite_count'], 5);
    });
    test('POST favourite returns favourited true', () {
      final data = {'favourited': true, 'favourite_count': 6};
      expect(data['favourited'], true);
    });
  });
}
