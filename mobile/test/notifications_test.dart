import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/api/models.dart';
import 'package:redchef/src/notifications/push_service.dart';

void main() {
  group('NotificationConfig', () {
    test('parses the server config payload', () {
      final c = NotificationConfig.fromJson({
        'enabled': true,
        'new_post_enabled': true,
        'web_push_enabled': false,
        'fcm_enabled': true,
        'vapid_public_key': 'BPublic',
      });
      expect(c.enabled, true);
      expect(c.newPostEnabled, true);
      expect(c.fcmEnabled, true);
      expect(c.webPushEnabled, false);
      expect(c.vapidPublicKey, 'BPublic');
      expect(c.pushSupported, true);
    });

    test('defaults to everything off when the server has no feature', () {
      const c = NotificationConfig();
      expect(c.enabled, false);
      expect(c.pushSupported, false);
    });

    test('treats numeric flags like the Go backend sends', () {
      final c = NotificationConfig.fromJson({'enabled': 1, 'fcm_enabled': 0});
      expect(c.enabled, true);
      expect(c.fcmEnabled, false);
    });
  });

  group('NotificationPrefs', () {
    test('parses and round-trips the preferences payload', () {
      final p = NotificationPrefs.fromJson({'user_id': 1, 'new_posts': true, 'email': false, 'push': true});
      expect(p.newPosts, true);
      expect(p.email, false);
      expect(p.push, true);
      expect(p.toJson(), {'new_posts': true, 'email': false, 'push': true});
    });

    test('defaults to all on', () {
      final p = NotificationPrefs.fromJson({});
      expect(p.newPosts, true);
      expect(p.email, true);
      expect(p.push, true);
    });

    test('copyWith changes one flag', () {
      const p = NotificationPrefs();
      final next = p.copyWith(push: false);
      expect(next.push, false);
      expect(next.newPosts, true);
    });
  });

  group('notification deep links', () {
    test('maps the web post path onto the app route', () {
      expect(routeForNotification({'url': '/posts/123'}), '/post/123');
      expect(routeForNotification({'url': '/posts/123?ref=push'}), '/post/123');
    });

    test('accepts an app route or trailing slash', () {
      expect(routeForNotification({'url': '/post/7'}), '/post/7');
      expect(routeForNotification({'url': '/posts/9/'}), '/post/9');
    });

    test('falls back to post_id when the url is missing', () {
      expect(routeForNotification({'post_id': '42'}), '/post/42');
    });

    test('ignores payloads without a target', () {
      expect(routeForNotification({}), isNull);
      expect(routeForNotification({'url': '/settings'}), '/settings');
    });
  });
}
