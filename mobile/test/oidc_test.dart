import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/utils/oidc.dart';

void main() {
  group('isSameOrigin', () {
    test('accepts the same scheme, host and port', () {
      expect(
        isSameOrigin('https://chef.example.com/login', 'https://chef.example.com'),
        isTrue,
      );
    });

    test('accepts a trailing slash on the base URL', () {
      expect(
        isSameOrigin('https://chef.example.com/', 'https://chef.example.com/'),
        isTrue,
      );
      expect(
        isSameOrigin('https://chef.example.com/callback', 'https://chef.example.com/'),
        isTrue,
      );
    });

    test('rejects a different port', () {
      expect(
        isSameOrigin('https://chef.example.com:8443/', 'https://chef.example.com'),
        isFalse,
      );
    });

    test('rejects http vs https even on the same host', () {
      expect(
        isSameOrigin('http://chef.example.com/', 'https://chef.example.com'),
        isFalse,
      );
    });

    test('compares hosts case-insensitively', () {
      expect(
        isSameOrigin('https://CHEF.Example.COM/', 'https://chef.example.com'),
        isTrue,
      );
    });

    test('treats default http and https ports as implicit', () {
      expect(
        isSameOrigin('http://chef.example.com:80/', 'http://chef.example.com'),
        isTrue,
      );
      expect(
        isSameOrigin('https://chef.example.com:443/', 'https://chef.example.com'),
        isTrue,
      );
      expect(
        isSameOrigin('https://chef.example.com:80/', 'https://chef.example.com'),
        isFalse,
      );
    });

    test('rejects different hosts', () {
      expect(
        isSameOrigin('https://idp.example.org/', 'https://chef.example.com'),
        isFalse,
      );
    });

    test('rejects malformed or relative URLs without throwing', () {
      expect(isSameOrigin('not a url', 'https://chef.example.com'), isFalse);
      expect(isSameOrigin('', 'https://chef.example.com'), isFalse);
      expect(isSameOrigin('https://chef.example.com', ''), isFalse);
      expect(isSameOrigin('://missing-scheme', 'https://chef.example.com'), isFalse);
      expect(isSameOrigin('/relative', 'https://chef.example.com'), isFalse);
    });
  });

  group('oidcErrorCodeFromUrl', () {
    test('extracts an error code from a same-origin redirect', () {
      expect(
        oidcErrorCodeFromUrl(
          'https://chef.example.com/?error=oidc_state',
          'https://chef.example.com',
        ),
        'oidc_state',
      );
    });

    test('ignores error codes from another origin', () {
      expect(
        oidcErrorCodeFromUrl(
          'https://idp.example.org/?error=oidc_state',
          'https://chef.example.com',
        ),
        isNull,
      );
    });

    test('returns null when there is no error parameter', () {
      expect(
        oidcErrorCodeFromUrl('https://chef.example.com/', 'https://chef.example.com'),
        isNull,
      );
    });
  });

  group('oidcErrorMessage', () {
    test('maps known server error codes to friendly messages', () {
      expect(oidcErrorMessage('oidc_expired'), contains('expired'));
      expect(oidcErrorMessage('oidc_state'), contains('security check'));
      expect(oidcErrorMessage('oidc_nonce'), contains('security check'));
      expect(oidcErrorMessage('oidc_provider'), contains('identity provider'));
      expect(oidcErrorMessage('oidc_exchange'), contains('identity provider'));
      expect(oidcErrorMessage('oidc_verify'), contains('verify'));
      expect(oidcErrorMessage('oidc_token'), contains('token'));
      expect(oidcErrorMessage('oidc_claims'), contains('invalid'));
      expect(oidcErrorMessage('oidc_email'), contains('verified email'));
      expect(oidcErrorMessage('oidc_user'), contains('account'));
      expect(oidcErrorMessage('oidc_session'), contains('Session creation failed'));
    });

    test('falls back to a generic message for unknown codes', () {
      expect(oidcErrorMessage('something_else'), 'Sign-in failed. Please try again.');
      expect(oidcErrorMessage(''), 'Sign-in failed. Please try again.');
    });
  });

  group('OidcResult', () {
    test('success carries the token', () {
      const result = OidcResult.success('token-123');
      expect(result.isSuccess, isTrue);
      expect(result.token, 'token-123');
      expect(result.error, isNull);
    });

    test('failure carries the message', () {
      const result = OidcResult.failure('Nope');
      expect(result.isSuccess, isFalse);
      expect(result.token, isNull);
      expect(result.error, 'Nope');
    });
  });
}
