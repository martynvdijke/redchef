import 'package:flutter_secure_storage/flutter_secure_storage.dart';

class SecureStore {
  static const _storage = FlutterSecureStorage();
  static const keyBaseUrl = 'base_url';
  static const keyCookie = 'session_cookie';
  static const keyRole = 'user_role';
  static const keyEmail = 'user_email';

  static Future<String?> getBaseUrl() => _storage.read(key: keyBaseUrl);
  static Future<void> setBaseUrl(String v) => _storage.write(key: keyBaseUrl, value: v);
  static Future<String?> getCookie() => _storage.read(key: keyCookie);
  static Future<void> setCookie(String v) => _storage.write(key: keyCookie, value: v);
  static Future<void> clearCookie() => _storage.delete(key: keyCookie);
  static Future<void> clearAll() async {
    await _storage.delete(key: keyCookie);
    await _storage.delete(key: keyRole);
    await _storage.delete(key: keyEmail);
  }

  static Future<String?> getRole() => _storage.read(key: keyRole);
  static Future<void> setRole(String v) => _storage.write(key: keyRole, value: v);
}
