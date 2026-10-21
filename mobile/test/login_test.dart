import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/api/api_client.dart';
import 'package:redchef/src/auth/auth_provider.dart';
import 'package:redchef/src/screens/login_screen.dart';

Dio _mockLoginDio({required bool oidcEnabled}) {
  final dio = Dio(BaseOptions(baseUrl: 'http://example.com'));
  dio.interceptors.add(InterceptorsWrapper(onRequest: (options, handler) {
    final path = options.uri.path;
    if (path.endsWith('/api/auth/oidc/status')) {
      return handler.resolve(Response(
        requestOptions: options,
        statusCode: 200,
        data: {'enabled': oidcEnabled},
      ));
    }
    if (path.endsWith('/api/auth/me')) {
      return handler.resolve(Response(
        requestOptions: options,
        statusCode: 200,
        data: {'authenticated': false},
      ));
    }
    return handler.next(options);
  }));
  return dio;
}

void main() {
  testWidgets('login screen shows Continue with SSO when OIDC is enabled', (tester) async {
    final api = ApiClient(dioInstance: _mockLoginDio(oidcEnabled: true));
    api.setBaseUrlSync('http://example.com');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: LoginScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Continue with SSO'), findsOneWidget);
    expect(find.text('Login with Authelia'), findsNothing);
  });

  testWidgets('login screen hides SSO when OIDC is disabled', (tester) async {
    final api = ApiClient(dioInstance: _mockLoginDio(oidcEnabled: false));
    api.setBaseUrlSync('http://example.com');

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: LoginScreen()),
      ),
    );
    await tester.pumpAndSettle();

    expect(find.text('Continue with SSO'), findsNothing);
  });
}
