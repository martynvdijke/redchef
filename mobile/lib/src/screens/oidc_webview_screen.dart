import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../utils/cookies.dart';
import '../utils/oidc.dart';

/// Hosts the server's OIDC authorization flow in an in-app WebView.
///
/// Pops with an [OidcResult] on success or failure, or `null` if the user backs
/// out. The session cookie is `HttpOnly`, so it cannot be read from JavaScript;
/// it is read from the native cookie store over a small platform channel
/// (`xyz.vandijke.redchef/cookies`).
class OidcWebViewScreen extends StatefulWidget {
  const OidcWebViewScreen({super.key, required this.baseUrl});

  final String baseUrl;

  static const MethodChannel cookieChannel = oidcCookieChannel;

  @override
  State<OidcWebViewScreen> createState() => _OidcWebViewScreenState();
}

class _OidcWebViewScreenState extends State<OidcWebViewScreen> {
  static const int _maxCookieFailures = 3;

  late final WebViewController _controller;
  bool _captured = false;
  bool _popped = false;
  bool _loading = true;
  String? _error;
  String? _currentUrl;
  int _cookieFailures = 0;

  String get _loginUrl => '${widget.baseUrl}/api/auth/oidc/login';

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageStarted: (_) {
            if (mounted) setState(() => _loading = true);
          },
          onUrlChange: (change) {
            final url = change.url;
            if (url == null || url.isEmpty) return;
            _currentUrl = url;
            _checkServerError(url);
          },
          onPageFinished: (url) {
            _currentUrl = url;
            if (!mounted) return;
            setState(() => _loading = false);
            if (_checkServerError(url)) return;
            _maybeCaptureSession();
          },
          onWebResourceError: (error) {
            // Ignore sub-resource failures and user-cancelled navigations.
            final description = error.description.toLowerCase();
            final cancelled = error.errorCode == -999 ||
                description.contains('cancel') ||
                description.contains('aborted');
            if (error.isForMainFrame == false || cancelled) return;
            _fail('Could not load the sign-in page. Check your connection and try again.');
          },
        ),
      )
      ..loadRequest(Uri.parse(_loginUrl));
  }

  bool _checkServerError(String url) {
    final code = oidcErrorCodeFromUrl(url, widget.baseUrl);
    if (code == null) return false;
    _fail(oidcErrorMessage(code));
    return true;
  }

  void _fail(String message) {
    if (!mounted || _popped) return;
    setState(() {
      _error = message;
      _loading = false;
    });
  }

  void _finish(OidcResult? result) {
    if (_popped || !mounted) return;
    _popped = true;
    Navigator.of(context).pop(result);
  }

  /// Pops with `null`, which the caller treats as a user cancellation.
  void _cancel() => _finish(null);

  Future<void> _maybeCaptureSession() async {
    if (_captured || _popped) return;
    final current = _currentUrl;
    // Never read the cookie while an identity provider page is loaded: an IdP
    // could otherwise plant a cookie named `session_token`.
    if (current == null || !isSameOrigin(current, widget.baseUrl)) return;

    try {
      final raw = await OidcWebViewScreen.cookieChannel
          .invokeMethod<String>('getCookies', {'url': widget.baseUrl});
      if (_captured || _popped) return;
      final token = parseCookieHeader(raw, 'session_token');
      if (token != null && token.isNotEmpty) {
        _captured = true;
        _finish(OidcResult.success(token));
      }
    } on MissingPluginException {
      _fail("In-app sign-in isn't available on this device.");
    } on PlatformException {
      _cookieFailures++;
      if (_cookieFailures >= _maxCookieFailures) {
        _fail('Could not read the sign-in session. Please try again.');
      }
      // Otherwise ignore and retry on the next navigation.
    }
  }

  void _retry() {
    if (!mounted) return;
    setState(() {
      _error = null;
      _loading = true;
      _cookieFailures = 0;
      _captured = false;
    });
    _controller.loadRequest(Uri.parse(_loginUrl));
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Sign in with SSO'),
        leading: IconButton(
          icon: const Icon(Icons.close),
          tooltip: 'Cancel',
          onPressed: _cancel,
        ),
      ),
      body: _error == null
          ? Stack(
              children: [
                WebViewWidget(controller: _controller),
                if (_loading) const LinearProgressIndicator(),
              ],
            )
          : Center(
              child: SingleChildScrollView(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(Icons.error_outline, size: 48, color: scheme.error),
                    const SizedBox(height: 16),
                    Text(
                      _error!,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodyLarge,
                    ),
                    const SizedBox(height: 20),
                    FilledButton.icon(
                      onPressed: _retry,
                      icon: const Icon(Icons.refresh),
                      label: const Text('Retry'),
                    ),
                  ],
                ),
              ),
            ),
    );
  }
}
