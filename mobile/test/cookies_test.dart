import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/utils/cookies.dart';

void main() {
  test('extracts a cookie from a header with several values', () {
    expect(
      parseCookieHeader('a=b; session_token=xyz; theme=dark', 'session_token'),
      'xyz',
    );
  });

  test('returns null when the cookie is missing', () {
    expect(parseCookieHeader('a=b; theme=dark', 'session_token'), isNull);
    expect(parseCookieHeader(null, 'session_token'), isNull);
    expect(parseCookieHeader('', 'session_token'), isNull);
  });

  test('returns null for an empty cookie value', () {
    expect(parseCookieHeader('session_token=; theme=dark', 'session_token'), isNull);
  });
}
