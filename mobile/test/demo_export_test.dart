import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/api/api_client.dart';
import 'package:redchef/src/auth/auth_provider.dart';
import 'package:redchef/src/screens/account_screen.dart';
import 'package:redchef/src/screens/login_screen.dart';

Dio _mockDio({bool demoEnabled = true}) {
  final dio = Dio(BaseOptions(baseUrl: 'http://example.com'));
  dio.interceptors.add(InterceptorsWrapper(onRequest: (options, handler) {
    final path = options.uri.path;
    if (path.endsWith('/api/auth/config')) {
      return handler.resolve(Response(
        requestOptions: options,
        statusCode: 200,
        data: {
          'demo': demoEnabled
              ? {
                  'enabled': true,
                  'email': 'demo@redchef.local',
                  'password': 'redchef-demo',
                }
              : {'enabled': false},
        },
      ));
    }
    if (path.endsWith('/api/auth/oidc/status')) {
      return handler.resolve(Response(
        requestOptions: options,
        statusCode: 200,
        data: {'enabled': false},
      ));
    }
    if (path.endsWith('/api/auth/me')) {
      return handler.resolve(Response(
        requestOptions: options,
        statusCode: 200,
        data: {'authenticated': false},
      ));
    }
    if (path.endsWith('/api/me/export')) {
      return handler.resolve(Response(
        requestOptions: options,
        statusCode: 200,
        data: '{"profile":{"email":"demo@redchef.local"}}',
      ));
    }
    return handler.next(options);
  }));
  return dio;
}

void main() {
  testWidgets('login screen shows demo account card when enabled',
      (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: LoginScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Shared demo account'), findsOneWidget);
    expect(find.text('demo@redchef.local'), findsOneWidget);
    expect(find.text('Use demo account'), findsOneWidget);
  });

  testWidgets('login screen hides demo card when disabled', (tester) async {
    final api = ApiClient(dioInstance: _mockDio(demoEnabled: false));
    api.setBaseUrlSync('http://example.com');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: LoginScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Shared demo account'), findsNothing);
    expect(find.text('Use demo account'), findsNothing);
  });

  testWidgets('account screen exports data into a copyable dialog',
      (tester) async {
    final api = ApiClient(dioInstance: _mockDio());
    api.setBaseUrlSync('http://example.com');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: AccountScreen()),
      ),
    );
    await tester.pumpAndSettle();

    final tile = find.text('Download my data');
    await tester.ensureVisible(tile);
    await tester.tap(tile);
    await tester.pumpAndSettle();

    expect(find.text('My data'), findsOneWidget);
    expect(find.textContaining('bytes'), findsOneWidget);
    expect(find.textContaining('"profile"'), findsOneWidget);
    expect(find.text('Copy'), findsOneWidget);
    expect(find.text('Close'), findsOneWidget);
  });
}
