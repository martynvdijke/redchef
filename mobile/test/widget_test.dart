import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:redchef/src/api/api_client.dart';
import 'package:redchef/src/auth/auth_provider.dart';
import 'package:redchef/src/screens/feed_screen.dart';
import 'package:redchef/src/screens/forgot_password_screen.dart';
import 'package:redchef/src/screens/reset_password_screen.dart';
import 'package:redchef/src/screens/account_screen.dart';

Dio _mockDio() {
  final dio = Dio(BaseOptions(baseUrl: 'http://example.com'));
  dio.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
    final path = o.uri.path;
    bool isTags = path == '/api/tags' || o.path.contains('/api/tags');
    bool isPosts = path == '/api/posts' || o.path.contains('/api/posts');
    bool isMe = path == '/api/auth/me' || o.path.contains('/api/auth/me');
    if (isTags) {
      return h.resolve(Response(requestOptions: o, statusCode: 200, data: [
        {'name': 'ramen', 'count': 3},
        {'name': 'sushi', 'count': 1},
      ]));
    }
    if (isPosts) {
      return h.resolve(Response(
        requestOptions: o,
        statusCode: 200,
        data: [
          {
            'id': 1,
            'title': 'Test Ramen',
            'description': 'Yummy',
            'media_type': 'photo',
            'filename': '1.jpg',
            'thumbnail': 'thumb.jpg',
            'locked': false,
            'processing': false,
            'created_at': '2026-01-01T12:00:00Z',
            'recipe': {},
            'tags': ['ramen'],
            'status': 'published',
            'media_url': '/uploads/1.jpg',
            'unlocked': true,
            'favourited': false,
            'favourite_count': 0,
            'tip_count': 0,
            'linked_posts': [],
          }
        ],
        headers: Headers.fromMap({'x-total-count': ['1']}),
      ));
    }
    if (isMe) {
      return h.resolve(Response(requestOptions: o, statusCode: 200, data: {'authenticated': false}));
    }
    return h.next(o);
  }));
  return dio;
}

void main() {
  testWidgets('feed shows post from mocked api', (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          apiClientProvider.overrideWithValue(api),
        ],
        child: const MaterialApp(home: FeedScreen()),
      ),
    );
    expect(find.byType(CircularProgressIndicator), findsWidgets);
    await tester.pumpAndSettle(const Duration(seconds: 2));
    expect(find.text('Test Ramen'), findsOneWidget);
    expect(find.text('#ramen'), findsOneWidget);
  });

  testWidgets('feed search and tag bar render', (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: FeedScreen()),
      ),
    );
    await tester.pumpAndSettle(const Duration(seconds: 2));
    expect(find.text('All'), findsOneWidget);
    expect(find.byIcon(Icons.search), findsOneWidget);
  });

  testWidgets('forgot password screen renders', (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: ForgotPasswordScreen()),
      ),
    );
    expect(find.text('Forgot password'), findsOneWidget);
    expect(find.text('Send reset link'), findsOneWidget);
    expect(find.byIcon(Icons.mail_outline), findsOneWidget);
  });

  testWidgets('reset password screen renders with token field', (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: ResetPasswordScreen()),
      ),
    );
    expect(find.text('Reset password'), findsWidgets);
    expect(find.text('Reset token *'), findsOneWidget);
    expect(find.text('New password *'), findsOneWidget);
  });

  testWidgets('account screen shows server and actions', (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');
    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: AccountScreen()),
      ),
    );
    expect(find.text('Account'), findsOneWidget);
    expect(find.text('API tokens'), findsOneWidget);
    expect(find.text('Change server'), findsOneWidget);
  });
}
