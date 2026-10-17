import 'package:dio/dio.dart';
import '../auth/storage.dart';

class ApiClient {
  final Dio dio;
  String? _baseUrl;
  String? _cookie;

  ApiClient({Dio? dioInstance})
      : dio = dioInstance ?? Dio(BaseOptions(connectTimeout: const Duration(seconds: 12), receiveTimeout: const Duration(seconds: 12))) {
    dio.interceptors.add(InterceptorsWrapper(
      onRequest: (options, handler) async {
        if (_cookie != null && _cookie!.isNotEmpty) {
          options.headers['Cookie'] = 'session_token=$_cookie';
        }
        handler.next(options);
      },
      onResponse: (response, handler) async {
        _captureCookie(response);
        handler.next(response);
      },
      onError: (e, handler) async {
        if (e.response != null) _captureCookie(e.response!);
        handler.next(e);
      },
    ));
  }

  void _captureCookie(Response r) {
    final setCookie = r.headers.map['set-cookie'] ?? r.headers.map['Set-Cookie'];
    if (setCookie == null) return;
    for (final c in setCookie) {
      final match = RegExp(r'session_token=([^;]+)').firstMatch(c);
      if (match != null) {
        final token = match.group(1)!;
        if (token.isEmpty) {
          _cookie = null;
          try { SecureStore.clearCookie(); } catch (_) {}
        } else {
          _cookie = token;
          try { SecureStore.setCookie(token); } catch (_) {}
        }
      }
    }
  }

  Future<void> init() async {
    try {
      _baseUrl = await SecureStore.getBaseUrl();
      _cookie = await SecureStore.getCookie();
    } catch (_) {}
    if (_baseUrl != null && _baseUrl!.isNotEmpty) {
      dio.options.baseUrl = _baseUrl!;
    }
  }

  String? get baseUrl {
    if (_baseUrl != null && _baseUrl!.isNotEmpty) return _baseUrl;
    final b = dio.options.baseUrl;
    if (b.isNotEmpty) return b;
    return null;
  }

  Future<void> setBaseUrl(String url) async {
    var v = url.trim();
    if (v.endsWith('/')) v = v.substring(0, v.length - 1);
    _baseUrl = v;
    dio.options.baseUrl = v;
    try {
      await SecureStore.setBaseUrl(v);
    } catch (_) {}
  }

  // For tests / direct assignment without secure storage.
  void setBaseUrlSync(String url) {
    var v = url.trim();
    if (v.endsWith('/')) v = v.substring(0, v.length - 1);
    _baseUrl = v;
    dio.options.baseUrl = v;
  }

  String resolveMedia(String? mediaUrl) {
    if (mediaUrl == null || mediaUrl.isEmpty) return '';
    if (mediaUrl.startsWith('http')) return mediaUrl;
    final base = baseUrl ?? '';
    if (base.isEmpty) return mediaUrl;
    return '$base$mediaUrl';
  }

  // Helpers that prepend baseUrl automatically via dio baseUrl, but allow absolute.
  Dio get client => dio;
}
