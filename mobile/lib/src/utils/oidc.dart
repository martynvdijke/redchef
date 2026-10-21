import 'package:flutter/services.dart';

/// Platform channel exposed by `MainActivity` to read the native WebView
/// cookie store. The session cookie is `HttpOnly`, so JavaScript cannot see it.
const MethodChannel oidcCookieChannel =
    MethodChannel('xyz.vandijke.redchef/cookies');

/// Outcome of the in-app OIDC flow.
///
/// A `null` result from the screen means the user cancelled.
class OidcResult {
  const OidcResult.success(this.token) : error = null;

  const OidcResult.failure(this.error) : token = null;

  /// Captured `session_token`, set on success.
  final String? token;

  /// User-facing error message, set on failure.
  final String? error;

  bool get isSuccess => token != null && token!.isNotEmpty;
}

int _effectivePort(Uri uri) {
  if (uri.hasPort) return uri.port;
  switch (uri.scheme.toLowerCase()) {
    case 'http':
      return 80;
    case 'https':
      return 443;
    default:
      return 0;
  }
}

/// Whether [url] has the same origin (scheme, host and port) as [baseUrl].
///
/// Used to make sure the `session_token` cookie is only captured from the
/// RedChef server, never from an identity provider page that happens to set a
/// cookie with the same name. Malformed or relative URLs return `false`
/// instead of throwing.
bool isSameOrigin(String url, String baseUrl) {
  final Uri? parsedUrl;
  final Uri? parsedBase;
  try {
    parsedUrl = Uri.tryParse(url.trim());
    parsedBase = Uri.tryParse(baseUrl.trim());
  } catch (_) {
    return false;
  }
  if (parsedUrl == null || parsedBase == null) return false;
  if (parsedUrl.scheme.isEmpty || parsedBase.scheme.isEmpty) return false;
  if (parsedUrl.host.isEmpty || parsedBase.host.isEmpty) return false;
  if (parsedUrl.scheme.toLowerCase() != parsedBase.scheme.toLowerCase()) {
    return false;
  }
  if (parsedUrl.host.toLowerCase() != parsedBase.host.toLowerCase()) {
    return false;
  }
  return _effectivePort(parsedUrl) == _effectivePort(parsedBase);
}

/// Returns the server's OIDC error code from a redirect URL (`/?error=…`).
///
/// Only same-origin URLs are considered, so an identity provider cannot spoof
/// an error redirect. Returns `null` when the URL is not a server redirect or
/// has no `error` query parameter.
String? oidcErrorCodeFromUrl(String url, String baseUrl) {
  if (!isSameOrigin(url, baseUrl)) return null;
  final uri = Uri.tryParse(url);
  if (uri == null) return null;
  final code = uri.queryParameters['error'];
  if (code == null || code.isEmpty) return null;
  return code;
}

/// Maps a server OIDC error code to a clear, user-facing message.
String oidcErrorMessage(String code) {
  switch (code) {
    case 'oidc_expired':
      return 'Your sign-in session expired. Please try again.';
    case 'oidc_state':
    case 'oidc_nonce':
      return 'The security check failed. Please sign in again.';
    case 'oidc_provider':
      return 'The identity provider returned an error. Please try again.';
    case 'oidc_exchange':
      return 'Could not complete sign-in with the identity provider. Please try again.';
    case 'oidc_verify':
      return 'Could not verify the sign-in response. Please try again.';
    case 'oidc_token':
      return 'Could not obtain a token from the identity provider. Please try again.';
    case 'oidc_claims':
      return 'The identity provider returned an invalid response. Please try again.';
    case 'oidc_email':
      return 'Your identity provider did not share a verified email address.';
    case 'oidc_user':
      return 'Could not find or create your account. Contact your administrator.';
    case 'oidc_session':
      return 'Session creation failed. Please try again.';
    default:
      return 'Sign-in failed. Please try again.';
  }
}

/// Clears cookies stored by the in-app WebView, so an identity provider
/// session is not left behind after logout.
///
/// Safe to call on platforms where the channel is not implemented.
Future<void> clearWebViewCookies() async {
  try {
    await oidcCookieChannel.invokeMethod<void>('clearCookies');
  } on MissingPluginException {
    // Channel unavailable (non-Android host); nothing to clear.
  } on PlatformException {
    // Best effort; logout must not fail because of cookie cleanup.
  } catch (_) {
    // Best effort.
  }
}
