import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

/// Compile-time defaults. The OTLP endpoint can also be supplied at runtime by
/// the server (`GET /api/otel-settings`); these values remain the fallback so a
/// `--dart-define` build still reports telemetry when the API is unreachable.
abstract final class TelemetryConfig {
  static const otelEndpoint = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_ENDPOINT',
  );
  static const otelHeaders = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_HEADERS',
  );
  static const serviceName = String.fromEnvironment(
    'OTEL_SERVICE_NAME',
    defaultValue: 'redchef-android',
  );
  static const serviceVersion = String.fromEnvironment(
    'OTEL_SERVICE_VERSION',
    defaultValue: '1.0.0',
  );
  static const umamiUrl = String.fromEnvironment('UMAMI_URL');
  static const umamiWebsiteId = String.fromEnvironment('UMAMI_WEBSITE_ID');

  static bool get otelEnabled => otelEndpoint.trim().isNotEmpty && !kIsWeb;
  static bool get umamiEnabled =>
      umamiUrl.trim().isNotEmpty && umamiWebsiteId.trim().isNotEmpty;
}

/// Allowlisted analytics events. Keys and values are enforced at the boundary,
/// so titles, tags, user names and search terms can never be sent.
const Map<String, Set<String>> umamiEventAllowlist = {
  'screen_view': {'screen'},
  'post_view': {'media_type'},
  'favourite_toggled': {'action'},
  'search': {'has_query'},
  'theme_changed': {'theme'},
  'sync_started': {},
};

const Set<String> _themes = {'system', 'light', 'dark'};
const Set<String> _actions = {'add', 'remove'};
const Set<String> _mediaTypes = {'photo', 'video'};

Map<String, Object>? sanitizeUmamiEvent(
  String event, [
  Map<String, Object?> props = const {},
]) {
  final allowed = umamiEventAllowlist[event];
  if (allowed == null) return null;
  final clean = <String, Object>{};
  for (final entry in props.entries) {
    if (!allowed.contains(entry.key)) continue;
    final value = entry.value;
    if (value is bool || value is num) {
      clean[entry.key] = value as Object;
      continue;
    }
    if (value is! String || value.length > 64) continue;
    if (entry.key == 'theme' && !_themes.contains(value)) continue;
    if (entry.key == 'action' && !_actions.contains(value)) continue;
    if (entry.key == 'media_type' && !_mediaTypes.contains(value)) continue;
    clean[entry.key] = value;
  }
  if (allowed.isNotEmpty && clean.isEmpty) return null;
  return clean;
}

String randomHex(int bytes) {
  final random = Random.secure();
  final buffer = StringBuffer();
  for (var i = 0; i < bytes; i++) {
    buffer.write(random.nextInt(256).toRadixString(16).padLeft(2, '0'));
  }
  return buffer.toString();
}

String buildTraceparent(String traceId, String spanId) =>
    '00-$traceId-$spanId-01';

/// Runtime OTLP configuration returned by the web app.
class OtelSettings {
  const OtelSettings({
    required this.endpoint,
    required this.enabled,
    required this.tracesEnabled,
    required this.metricsEnabled,
    required this.logsEnabled,
  });

  final String endpoint;
  final bool enabled;
  final bool tracesEnabled;
  final bool metricsEnabled;
  final bool logsEnabled;

  static const OtelSettings disabled = OtelSettings(
    endpoint: '',
    enabled: false,
    tracesEnabled: false,
    metricsEnabled: false,
    logsEnabled: false,
  );

  bool get tracingEnabled =>
      enabled && tracesEnabled && endpoint.trim().isNotEmpty;

  /// Tolerant parse: accepts `endpoint`/`otel_endpoint`, `enabled`/
  /// `otel_enabled` and the per-signal flags, with booleans as bool, number or
  /// string. An empty endpoint disables everything.
  factory OtelSettings.fromJson(Map<String, dynamic> json) {
    final endpoint = _string(json, const ['endpoint', 'otel_endpoint']).trim();
    final enabled = _bool(json, const ['enabled', 'otel_enabled']) ?? false;
    final traces = _bool(json, const ['traces_enabled']) ?? enabled;
    final metrics = _bool(json, const ['metrics_enabled']) ?? enabled;
    final logs = _bool(json, const ['logs_enabled']) ?? enabled;
    final active = enabled && endpoint.isNotEmpty;
    return OtelSettings(
      endpoint: endpoint,
      enabled: active,
      tracesEnabled: active && traces,
      metricsEnabled: active && metrics,
      logsEnabled: active && logs,
    );
  }

  static String _string(Map<String, dynamic> json, List<String> keys) {
    for (final key in keys) {
      final value = json[key];
      if (value is String) return value;
      if (value != null) return value.toString();
    }
    return '';
  }

  static bool? _bool(Map<String, dynamic> json, List<String> keys) {
    for (final key in keys) {
      if (!json.containsKey(key)) continue;
      final value = json[key];
      if (value is bool) return value;
      if (value is num) return value != 0;
      if (value is String) {
        final normalized = value.trim().toLowerCase();
        if (normalized == 'true' || normalized == '1' || normalized == 'yes') {
          return true;
        }
        if (normalized == 'false' || normalized == '0' || normalized == 'no') {
          return false;
        }
      }
      return null;
    }
    return null;
  }
}

/// Fetches runtime OTLP settings from the server. Never throws: offline,
/// timeouts and non-200 responses all return `null`.
Future<OtelSettings?> fetchOtelSettings(
  String baseUrl, {
  Dio? dio,
  Duration timeout = const Duration(seconds: 3),
}) async {
  final base = baseUrl.trim().replaceAll(RegExp(r'/+$'), '');
  if (base.isEmpty) return null;
  final ownDio = dio == null;
  final client = dio ??
      Dio(
        BaseOptions(
          connectTimeout: timeout,
          receiveTimeout: timeout,
          sendTimeout: timeout,
        ),
      );
  try {
    final response = await client.get<dynamic>(
      '$base/api/otel-settings',
      options: Options(
        responseType: ResponseType.json,
        receiveTimeout: timeout,
        validateStatus: (_) => true,
      ),
    );
    if (response.statusCode != 200) return null;
    final data = response.data;
    if (data is Map) return OtelSettings.fromJson(_asStringMap(data));
    if (data is String && data.isNotEmpty) {
      final decoded = jsonDecode(data);
      if (decoded is Map) return OtelSettings.fromJson(_asStringMap(decoded));
    }
    return null;
  } catch (_) {
    return null;
  } finally {
    if (ownDio) client.close(force: true);
  }
}

Map<String, dynamic> _asStringMap(Map<dynamic, dynamic> data) =>
    data.map((key, value) => MapEntry(key.toString(), value));

class TraceSpan {
  TraceSpan({
    required this.name,
    required this.attributes,
    required this.traceId,
    required this.spanId,
    required this.startNanos,
    this.kind = 1,
  });

  final String name;
  final Map<String, Object> attributes;
  final String traceId;
  final String spanId;
  final int startNanos;
  final int kind;
  bool _ended = false;
  int? statusCode;
  String error = '';

  String get traceparent => buildTraceparent(traceId, spanId);

  void end({
    int? httpStatus,
    Object? failure,
    Map<String, Object> extra = const {},
  }) {
    if (_ended) return;
    _ended = true;
    statusCode = httpStatus;
    if (failure != null) error = failure.toString();
    final attrs = <String, Object>{...attributes, ...extra};
    if (httpStatus != null) attrs['http.response.status_code'] = httpStatus;
    if (error.isNotEmpty) attrs['error.message'] = _short(error);
    Telemetry.instance._finish(
      this,
      attrs,
      error.isNotEmpty || (httpStatus != null && httpStatus >= 500),
    );
  }

  String _short(String value) =>
      value.length > 200 ? '${value.substring(0, 200)}…' : value;
}

Map<String, Object?> buildOtlpPayload({
  required String serviceName,
  required String serviceVersion,
  required TraceSpan span,
  required Map<String, Object> attributes,
  required bool failed,
  required int endNanos,
}) {
  List<Map<String, Object?>> attrs(Map<String, Object> values) => values.entries
      .map(
        (entry) => {
          'key': entry.key,
          'value': switch (entry.value) {
            final bool value => {'boolValue': value},
            final int value => {'intValue': value.toString()},
            final double value => {'doubleValue': value},
            _ => {'stringValue': entry.value.toString()},
          },
        },
      )
      .toList();

  return {
    'resourceSpans': [
      {
        'resource': {
          'attributes': attrs({
            'service.name': serviceName,
            'service.version': serviceVersion,
            'telemetry.sdk.name': 'redchef-mobile',
            'telemetry.sdk.language': 'dart',
          }),
        },
        'scopeSpans': [
          {
            'scope': {'name': 'redchef.mobile', 'version': serviceVersion},
            'spans': [
              {
                'traceId': span.traceId,
                'spanId': span.spanId,
                'name': span.name,
                'kind': span.kind,
                'startTimeUnixNano': span.startNanos.toString(),
                'endTimeUnixNano': endNanos.toString(),
                'attributes': attrs(attributes),
                'status': {'code': failed ? 2 : 1},
              },
            ],
          },
        ],
      },
    ],
  };
}

class Telemetry {
  Telemetry._();

  static final Telemetry instance = Telemetry._();

  Dio? _dio;
  Timer? _timer;
  final List<Map<String, Object?>> _queue = [];
  bool _started = false;
  Future<void>? _starting;
  String _tracesEndpoint = '';
  Map<String, String> _headers = const {};

  bool get otelEnabled => _tracesEndpoint.isNotEmpty;
  bool get umamiEnabled => TelemetryConfig.umamiEnabled;

  /// Starts OTLP export. An explicit [endpoint] wins over the dart-define
  /// default; empty means disabled. Safe to call repeatedly.
  Future<void> start({String? endpoint, Map<String, String>? headers}) async {
    if (_started) return;
    _started = true;
    if (kIsWeb) return;
    final raw = (endpoint ?? TelemetryConfig.otelEndpoint).trim();
    if (raw.isEmpty) return;
    _dio = Dio(
      BaseOptions(
        connectTimeout: const Duration(seconds: 5),
        receiveTimeout: const Duration(seconds: 5),
        sendTimeout: const Duration(seconds: 5),
      ),
    );
    _tracesEndpoint = normalizeEndpoint(raw);
    _headers = headers ?? parseOtelHeaders(TelemetryConfig.otelHeaders);
    _timer = Timer.periodic(const Duration(seconds: 5), (_) => _flush());
    beginSpan('app.start').end();
  }

  /// Asks the server for runtime OTLP settings and starts exporting when
  /// tracing is enabled. Falls back to the dart-define endpoint, otherwise
  /// stays disabled. Never throws and never blocks startup for long.
  Future<void> startFromServer(String baseUrl) =>
      _starting ??= _startFromServer(baseUrl);

  Future<void> _startFromServer(String baseUrl) async {
    try {
      final settings = await fetchOtelSettings(baseUrl);
      if (settings != null && settings.tracingEnabled) {
        await start(endpoint: settings.endpoint);
        return;
      }
    } catch (_) {
      // Settings are best-effort; fall through to the compile-time default.
    }
    if (TelemetryConfig.otelEnabled) {
      await start();
      return;
    }
    _started = true;
  }

  TraceSpan beginSpan(
    String name, {
    Map<String, Object> attributes = const {},
    int kind = 1,
  }) {
    return TraceSpan(
      name: name,
      attributes: attributes,
      traceId: randomHex(16),
      spanId: randomHex(8),
      startNanos: DateTime.now().microsecondsSinceEpoch * 1000,
      kind: kind,
    );
  }

  void screen(String name) {
    final span = beginSpan('screen $name', attributes: {'screen.name': name});
    span.end();
    track('screen_view', {'screen': name});
  }

  void track(String event, [Map<String, Object?> props = const {}]) {
    final clean = sanitizeUmamiEvent(event, props);
    if (clean == null || !TelemetryConfig.umamiEnabled) return;
    final client = _dio ??= Dio();
    final base = TelemetryConfig.umamiUrl.replaceAll(RegExp(r'/+$'), '');
    final uri = Uri.tryParse('$base/api/send');
    if (uri == null) return;
    final body = {
      'type': 'event',
      'payload': {
        'website': TelemetryConfig.umamiWebsiteId,
        'hostname': 'redchef.app',
        'url': '/app/$event',
        'event': event,
        'data': clean,
      },
    };
    unawaited(
      client
          .post<dynamic>(
            uri.toString(),
            data: jsonEncode(body),
            options: Options(contentType: Headers.jsonContentType),
          )
          .then((_) {}, onError: (Object _) {}),
    );
  }

  void _finish(TraceSpan span, Map<String, Object> attributes, bool failed) {
    if (!otelEnabled) return;
    final payload = buildOtlpPayload(
      serviceName: TelemetryConfig.serviceName,
      serviceVersion: TelemetryConfig.serviceVersion,
      span: span,
      attributes: attributes,
      failed: failed,
      endNanos: DateTime.now().microsecondsSinceEpoch * 1000,
    );
    _queue.add(payload);
    if (_queue.length >= 20) _flush();
  }

  void _flush() {
    if (_queue.isEmpty || _dio == null || _tracesEndpoint.isEmpty) return;
    final spans = <Map<String, Object?>>[];
    for (final payload in _queue) {
      final resourceSpans = payload['resourceSpans'];
      if (resourceSpans is List && resourceSpans.isNotEmpty) {
        final resource = resourceSpans.first;
        if (resource is Map) {
          final scopeSpans = resource['scopeSpans'];
          if (scopeSpans is List && scopeSpans.isNotEmpty) {
            final scope = scopeSpans.first;
            if (scope is Map && scope['spans'] is List) {
              spans.addAll(
                (scope['spans'] as List).cast<Map<String, Object?>>(),
              );
            }
          }
        }
      }
    }
    _queue.clear();
    if (spans.isEmpty) return;
    final payload = {
      'resourceSpans': [
        {
          'resource': {
            'attributes': [
              {
                'key': 'service.name',
                'value': {'stringValue': TelemetryConfig.serviceName},
              },
              {
                'key': 'service.version',
                'value': {'stringValue': TelemetryConfig.serviceVersion},
              },
            ],
          },
          'scopeSpans': [
            {
              'scope': {'name': 'redchef.mobile'},
              'spans': spans,
            },
          ],
        },
      ],
    };
    unawaited(
      _dio!
          .post<dynamic>(
            _tracesEndpoint,
            data: jsonEncode(payload),
            options: Options(
              contentType: Headers.jsonContentType,
              headers: _headers,
              validateStatus: (_) => true,
            ),
          )
          .then(
            (response) {
              if ((response.statusCode ?? 0) >= 400 && kDebugMode) {
                debugPrint('OTLP export failed: ${response.statusCode}');
              }
            },
            onError: (Object error) {
              if (kDebugMode) debugPrint('OTLP export failed: $error');
            },
          ),
    );
  }

  Future<void> shutdown() async {
    _timer?.cancel();
    _flush();
    await Future<void>.delayed(const Duration(milliseconds: 200));
    _dio?.close(force: true);
    _dio = null;
  }

  /// Appends `/v1/traces` unless the endpoint already names the traces path.
  static String normalizeEndpoint(String raw) {
    final base = raw.trim().replaceAll(RegExp(r'/+$'), '');
    if (base.isEmpty) return '';
    if (base.endsWith('/v1/traces')) return base;
    return '$base/v1/traces';
  }

  /// Parses the OTLP `key=value,key2=value2` header env format.
  static Map<String, String> parseOtelHeaders(String raw) {
    final out = <String, String>{};
    for (final pair in raw.split(',')) {
      final idx = pair.indexOf('=');
      if (idx <= 0) continue;
      final key = pair.substring(0, idx).trim();
      var value = pair.substring(idx + 1).trim();
      try {
        value = Uri.decodeComponent(value);
      } catch (_) {}
      if (key.isNotEmpty && value.isNotEmpty) out[key] = value;
    }
    return out;
  }
}
