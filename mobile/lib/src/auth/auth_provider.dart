import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import '../api/api_client.dart';
import '../api/models.dart';
import '../utils/oidc.dart';
import 'storage.dart';

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient());

final baseUrlProvider = StateProvider<String?>((ref) => null);

class AuthState {
  final bool loading;
  final MeResponse? me;
  final String? error;
  const AuthState({this.loading = false, this.me, this.error});
  bool get authenticated => me?.authenticated == true;
  bool get isAdmin => me?.isAdmin == true;
}

class AuthNotifier extends StateNotifier<AuthState> {
  final ApiClient api;
  AuthNotifier(this.api) : super(const AuthState(loading: true));

  Future<void> init() async {
    await api.init();
    await refreshMe();
  }

  Future<void> refreshMe() async {
    if (api.baseUrl == null || api.baseUrl!.isEmpty) {
      state = const AuthState(loading: false, me: null);
      return;
    }
    try {
      state = AuthState(loading: true, me: state.me);
      final r = await api.dio.get('/api/auth/me');
      final data = r.data is Map ? (r.data as Map).cast<String, dynamic>() : <String, dynamic>{'authenticated': false};
      state = AuthState(me: MeResponse.fromJson(data));
      if (state.me?.role != null) {
        try { await SecureStore.setRole(state.me!.role!); } catch (_) {}
      }
    } on DioException catch (e) {
      if (e.response?.statusCode == 401) {
        state = AuthState(me: MeResponse(authenticated: false));
      } else {
        state = AuthState(me: MeResponse(authenticated: false), error: e.message);
      }
    } catch (_) {
      state = AuthState(me: MeResponse(authenticated: false));
    }
  }

  Future<void> setBaseUrl(String url) async {
    await api.setBaseUrl(url);
    await refreshMe();
  }

  Future<void> login(String email, String password) async {
    state = AuthState(loading: true, me: state.me);
    final r = await api.dio.post('/api/auth/login', data: {'email': email, 'password': password});
    if (r.data is Map && r.data['role'] != null) {
      try { await SecureStore.setRole(r.data['role'].toString()); } catch (_) {}
    }
    await refreshMe();
  }

  Future<void> register(String email, String password) async {
    state = AuthState(loading: true, me: state.me);
    final r = await api.dio.post('/api/auth/register', data: {'email': email, 'password': password});
    if (r.data is Map && r.data['role'] != null) {
      try { await SecureStore.setRole(r.data['role'].toString()); } catch (_) {}
    }
    await refreshMe();
  }

  Future<void> logout() async {
    final wasOidc = await checkOidcEnabled();
    try {
      await api.dio.post('/api/auth/logout');
    } catch (_) {}
    if (wasOidc) {
      final base = api.baseUrl;
      if (base != null && base.isNotEmpty) {
        try {
          final uri = Uri.parse('$base/api/auth/oidc/logout');
          await launchUrl(uri, mode: LaunchMode.externalApplication);
        } catch (_) {}
      }
    }
    // Drop any identity provider session kept by the in-app WebView.
    await clearWebViewCookies();
    try { await SecureStore.clearAll(); } catch (_) {}
    state = AuthState(me: MeResponse(authenticated: false));
    await refreshMe();
  }

  Future<bool> checkOidcEnabled() async {
    try {
      final r = await api.dio.get('/api/auth/oidc/status');
      return r.data['enabled'] == true;
    } catch (_) {
      return false;
    }
  }

  Future<bool> validateServerUrl(String url) async {
    final err = await validateServerUrlDetailed(url);
    return err == null;
  }

  /// Returns null if valid RedChef server, otherwise human-readable error.
  Future<String?> validateServerUrlDetailed(String url) async {
    var v = url.trim();
    if (v.endsWith('/')) v = v.substring(0, v.length - 1);
    if (v.isEmpty) return 'Enter your RedChef server URL';
    if (!v.startsWith('http://') && !v.startsWith('https://')) {
      v = 'https://$v';
    }
    try {
      final dio = Dio(BaseOptions(connectTimeout: const Duration(seconds: 6), receiveTimeout: const Duration(seconds: 6)));
      final r = await dio.get('$v/api/setup/status', options: Options(validateStatus: (_) => true));
      if (r.statusCode != null && r.statusCode! >= 200 && r.statusCode! < 500) {
        // 2xx-4xx means server responded; check body looks like RedChef if available
        if (r.data is Map) {
          return null; // consider valid - we got a JSON from RedChef setup
        }
        if (r.statusCode! < 400) return null;
        // 4xx still means server reachable, but may not be setup endpoint - try /api/auth/me
      }
      if (r.statusCode == 404) {
        // Not a RedChef server (no setup route)
        final r2 = await dio.get('$v/api/auth/me', options: Options(validateStatus: (_) => true));
        if (r2.statusCode == 404) {
          return "This URL doesn't look like a RedChef server — no RedChef API found (tried /api/setup/status).";
        }
        if (r2.statusCode != null && r2.statusCode! < 500) return null;
        return "This URL doesn't look like a RedChef server.";
      }
      if (r.statusCode != null && r.statusCode! >= 500) {
        return 'Server error (${r.statusCode}) — is this a RedChef instance?';
      }
      // Fallback: try /api/auth/me
      final r2 = await dio.get('$v/api/auth/me', options: Options(validateStatus: (_) => true));
      if (r2.statusCode != null && r2.statusCode! < 500) return null;
      return "Could not confirm RedChef server at this URL.";
    } catch (e) {
      return 'Could not reach server — check URL and network.';
    }
  }

  Future<String> forgotPassword(String email) async {
    final r = await api.dio.post('/api/auth/forgot', data: {'email': email});
    final data = r.data;
    if (data is Map && data['message'] != null) return data['message'].toString();
    return 'If that email exists, a reset link has been sent.';
  }

  Future<String> resetPassword({required String token, required String password, String? confirmPassword}) async {
    final payload = {
      'token': token,
      'password': password,
      if (confirmPassword != null) 'confirm_password': confirmPassword,
    };
    final r = await api.dio.post('/api/auth/reset', data: payload);
    final data = r.data;
    if (data is Map && data['message'] != null) return data['message'].toString();
    return 'Password reset successful.';
  }

  // Tips
  Future<TipResult> tipPost(int postId, int amountCents) async {
    final r = await api.dio.post('/api/posts/$postId/tip', data: {'amount_cents': amountCents});
    return TipResult.fromJson((r.data as Map).cast<String, dynamic>());
  }

  // Admin
  Future<List<Post>> fetchAdminPosts({String? status}) async {
    final r = await api.dio.get('/api/admin/posts', queryParameters: {if (status != null) 'status': status});
    final list = r.data as List;
    return list.map((e) => Post.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<void> deleteAdminPost(int id) async {
    await api.dio.delete('/api/admin/posts/$id');
  }

  // Tokens
  Future<List<ApiToken>> fetchTokens() async {
    final r = await api.dio.get('/api/tokens');
    final list = r.data as List;
    return list.map((e) => ApiToken.fromJson(e as Map<String, dynamic>)).toList();
  }

  Future<ApiToken> createToken(String name) async {
    final r = await api.dio.post('/api/tokens', data: {'name': name});
    return ApiToken.fromJson((r.data as Map).cast<String, dynamic>());
  }

  Future<void> revokeToken(int id) async {
    await api.dio.delete('/api/tokens/$id');
  }

  Future<ApiToken> rotateToken(int id) async {
    final r = await api.dio.post('/api/tokens/$id/rotate');
    return ApiToken.fromJson((r.data as Map).cast<String, dynamic>());
  }

  // Favourite toggle helpers
  Future<Map<String, dynamic>> favouritePost(int postId) async {
    final r = await api.dio.post('/api/posts/$postId/favourite');
    return (r.data as Map).cast<String, dynamic>();
  }

  Future<Map<String, dynamic>> unfavouritePost(int postId) async {
    final r = await api.dio.delete('/api/favourites/$postId');
    return (r.data as Map).cast<String, dynamic>();
  }
}

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  final api = ref.watch(apiClientProvider);
  return AuthNotifier(api);
});
