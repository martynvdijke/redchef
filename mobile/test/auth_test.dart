import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/api/api_client.dart';
import 'package:redchef/src/auth/auth_provider.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

Dio mockAuthDio({required bool authenticated, bool oidcEnabled = false}) {
  final dio = Dio(BaseOptions(baseUrl: 'http://example.com'));
  dio.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
    final path = o.uri.path;
    if (path == '/api/auth/me' || o.path == '/api/auth/me' || o.path.endsWith('/api/auth/me')) {
      if (authenticated) {
        return h.resolve(Response(requestOptions: o, statusCode: 200, data: {
          'authenticated': true,
          'id': 1,
          'email': 'admin@example.com',
          'role': 'admin',
          'paid': true,
        }));
      } else {
        return h.resolve(Response(requestOptions: o, statusCode: 200, data: {'authenticated': false}));
      }
    }
    if (path == '/api/auth/oidc/status' || o.path == '/api/auth/oidc/status' || o.path.endsWith('/api/auth/oidc/status')) {
      return h.resolve(Response(requestOptions: o, statusCode: 200, data: {'enabled': oidcEnabled}));
    }
    return h.next(o);
  }));
  return dio;
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('AuthNotifier refreshMe sets authenticated state', () async {
    final api = ApiClient(dioInstance: mockAuthDio(authenticated: true));
    api.setBaseUrlSync('http://example.com');
    final container = ProviderContainer(overrides: [apiClientProvider.overrideWithValue(api)]);
    addTearDown(container.dispose);
    final notifier = container.read(authProvider.notifier);
    await notifier.refreshMe();
    final state = container.read(authProvider);
    expect(state.authenticated, true);
    expect(state.isAdmin, true);
  });

  test('AuthNotifier handles anonymous', () async {
    final api = ApiClient(dioInstance: mockAuthDio(authenticated: false));
    api.setBaseUrlSync('http://example.com');
    final container = ProviderContainer(overrides: [apiClientProvider.overrideWithValue(api)]);
    addTearDown(container.dispose);
    final notifier = container.read(authProvider.notifier);
    await notifier.refreshMe();
    expect(container.read(authProvider).authenticated, false);
  });

  test('ApiClient resolveMedia prepends baseUrl', () {
    final api = ApiClient();
    api.setBaseUrlSync('https://example.com');
    expect(api.resolveMedia('/uploads/1.jpg'), 'https://example.com/uploads/1.jpg');
    expect(api.resolveMedia(null), '');
    expect(api.resolveMedia('https://cdn.com/img.jpg'), 'https://cdn.com/img.jpg');
  });

  test('ApiClient baseUrl getter returns dio baseUrl', () {
    final api = ApiClient();
    expect(api.baseUrl, isNull);
    api.setBaseUrlSync('http://example.com');
    expect(api.baseUrl, 'http://example.com');
  });
}
