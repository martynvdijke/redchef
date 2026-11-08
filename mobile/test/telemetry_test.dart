import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:redchef/src/telemetry/telemetry.dart';

/// Serves canned responses without touching the network.
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this.handler);

  final Future<ResponseBody> Function(RequestOptions options) handler;
  RequestOptions? lastRequest;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    lastRequest = options;
    return handler(options);
  }

  @override
  void close({bool force = false}) {}
}

Dio _dioWith(_FakeAdapter adapter) => Dio()..httpClientAdapter = adapter;

ResponseBody _json(Object body, [int status = 200]) => ResponseBody.fromString(
      jsonEncode(body),
      status,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );

void main() {
  group('normalizeEndpoint', () {
    test('appends the traces path', () {
      expect(
        Telemetry.normalizeEndpoint('http://collector:4318'),
        'http://collector:4318/v1/traces',
      );
    });

    test('trims whitespace and trailing slashes', () {
      expect(
        Telemetry.normalizeEndpoint('  https://otel.example/  '),
        'https://otel.example/v1/traces',
      );
    });

    test('keeps an existing traces path', () {
      expect(
        Telemetry.normalizeEndpoint('https://otel.example/v1/traces/'),
        'https://otel.example/v1/traces',
      );
    });

    test('returns empty for an empty endpoint', () {
      expect(Telemetry.normalizeEndpoint('   '), '');
    });
  });

  group('parseOtelHeaders', () {
    test('parses comma separated pairs and decodes values', () {
      expect(
        Telemetry.parseOtelHeaders(
          'Authorization=Bearer%20abc,X-Tenant=acme',
        ),
        {'Authorization': 'Bearer abc', 'X-Tenant': 'acme'},
      );
    });

    test('ignores malformed pairs', () {
      expect(
        Telemetry.parseOtelHeaders('=bogus,noequals,Good=yes'),
        {'Good': 'yes'},
      );
    });

    test('returns empty for an empty string', () {
      expect(Telemetry.parseOtelHeaders(''), isEmpty);
    });
  });

  group('buildOtlpPayload', () {
    test('emits an OTLP JSON resource/scope span shape', () {
      final span = TraceSpan(
        name: 'http.request',
        attributes: const {'http.method': 'GET'},
        traceId: 'a' * 32,
        spanId: 'b' * 16,
        startNanos: 1000,
      );
      final payload = buildOtlpPayload(
        serviceName: 'redchef-android',
        serviceVersion: '1.2.3',
        span: span,
        attributes: const {'http.method': 'GET', 'http.response.status_code': 200},
        failed: false,
        endNanos: 2000,
      );

      final resourceSpan = (payload['resourceSpans'] as List).first as Map;
      final resourceAttrs = ((resourceSpan['resource'] as Map)['attributes'] as List)
          .cast<Map>();
      expect(
        resourceAttrs.firstWhere((a) => a['key'] == 'service.name')['value'],
        {'stringValue': 'redchef-android'},
      );
      expect(
        resourceAttrs.firstWhere((a) => a['key'] == 'telemetry.sdk.name')['value'],
        {'stringValue': 'redchef-mobile'},
      );

      final scope = (resourceSpan['scopeSpans'] as List).first as Map;
      expect((scope['scope'] as Map)['name'], 'redchef.mobile');
      final exported = (scope['spans'] as List).first as Map;
      expect(exported['traceId'], 'a' * 32);
      expect(exported['spanId'], 'b' * 16);
      expect(exported['name'], 'http.request');
      expect(exported['kind'], 1);
      expect(exported['startTimeUnixNano'], '1000');
      expect(exported['endTimeUnixNano'], '2000');
      expect((exported['status'] as Map)['code'], 1);
      expect(exported['attributes'], isA<List>());
    });

    test('marks failures with status code 2', () {
      final span = TraceSpan(
        name: 'http.request',
        attributes: const {},
        traceId: 'a' * 32,
        spanId: 'b' * 16,
        startNanos: 1,
      );
      final payload = buildOtlpPayload(
        serviceName: 'redchef-android',
        serviceVersion: '1.0.0',
        span: span,
        attributes: const {},
        failed: true,
        endNanos: 2,
      );
      final scope =
          (((payload['resourceSpans'] as List).first as Map)['scopeSpans'] as List)
              .first as Map;
      final exported = (scope['spans'] as List).first as Map;
      expect((exported['status'] as Map)['code'], 2);
    });
  });

  group('OtelSettings.fromJson', () {
    test('reads the canonical server payload', () {
      final settings = OtelSettings.fromJson({
        'endpoint': 'http://collector:4318',
        'enabled': true,
        'traces_enabled': true,
        'metrics_enabled': false,
        'logs_enabled': false,
      });
      expect(settings.endpoint, 'http://collector:4318');
      expect(settings.enabled, isTrue);
      expect(settings.tracingEnabled, isTrue);
      expect(settings.metricsEnabled, isFalse);
      expect(settings.logsEnabled, isFalse);
    });

    test('accepts the otel_* aliases', () {
      final settings = OtelSettings.fromJson({
        'otel_endpoint': 'https://otel.example',
        'otel_enabled': true,
        'traces_enabled': true,
      });
      expect(settings.endpoint, 'https://otel.example');
      expect(settings.enabled, isTrue);
      expect(settings.tracingEnabled, isTrue);
    });

    test('defaults per-signal flags to enabled', () {
      final settings = OtelSettings.fromJson({
        'endpoint': 'http://collector:4318',
        'enabled': true,
      });
      expect(settings.tracesEnabled, isTrue);
      expect(settings.metricsEnabled, isTrue);
      expect(settings.logsEnabled, isTrue);
    });

    test('accepts string and numeric booleans', () {
      final settings = OtelSettings.fromJson({
        'endpoint': 'http://collector:4318',
        'enabled': 'true',
        'traces_enabled': 1,
        'metrics_enabled': 'false',
        'logs_enabled': '0',
      });
      expect(settings.enabled, isTrue);
      expect(settings.tracesEnabled, isTrue);
      expect(settings.metricsEnabled, isFalse);
      expect(settings.logsEnabled, isFalse);
    });

    test('an empty endpoint disables everything', () {
      final settings = OtelSettings.fromJson({
        'endpoint': '',
        'enabled': true,
        'traces_enabled': true,
      });
      expect(settings.enabled, isFalse);
      expect(settings.tracesEnabled, isFalse);
      expect(settings.metricsEnabled, isFalse);
      expect(settings.logsEnabled, isFalse);
      expect(settings.tracingEnabled, isFalse);
    });

    test('missing fields degrade to disabled', () {
      final settings = OtelSettings.fromJson({});
      expect(settings.enabled, isFalse);
      expect(settings.tracingEnabled, isFalse);
    });
  });

  group('fetchOtelSettings', () {
    test('returns parsed settings on 200', () async {
      final adapter = _FakeAdapter(
        (_) async => _json({
          'endpoint': 'https://otel.example',
          'enabled': true,
          'traces_enabled': true,
          'metrics_enabled': false,
          'logs_enabled': false,
        }),
      );
      final settings = await fetchOtelSettings(
        'https://redchef.example/',
        dio: _dioWith(adapter),
      );

      expect(adapter.lastRequest?.uri.toString(),
          'https://redchef.example/api/otel-settings');
      expect(settings, isNotNull);
      expect(settings!.endpoint, 'https://otel.example');
      expect(settings.tracingEnabled, isTrue);
    });

    test('returns null on a non-200 response', () async {
      final adapter = _FakeAdapter((_) async => _json({'endpoint': 'x'}, 500));
      expect(
        await fetchOtelSettings('https://redchef.example', dio: _dioWith(adapter)),
        isNull,
      );
    });

    test('returns null on a transport error', () async {
      final adapter = _FakeAdapter(
        (options) async => throw DioException(
          requestOptions: options,
          type: DioExceptionType.connectionTimeout,
        ),
      );
      expect(
        await fetchOtelSettings('https://redchef.example', dio: _dioWith(adapter)),
        isNull,
      );
    });

    test('returns null on malformed JSON', () async {
      final adapter = _FakeAdapter(
        (_) async => ResponseBody.fromString(
          'not json',
          200,
          headers: {
            Headers.contentTypeHeader: [Headers.jsonContentType],
          },
        ),
      );
      expect(
        await fetchOtelSettings('https://redchef.example', dio: _dioWith(adapter)),
        isNull,
      );
    });

    test('returns null for an empty base URL', () async {
      final adapter = _FakeAdapter((_) async => _json({'endpoint': 'x'}));
      expect(await fetchOtelSettings('  ', dio: _dioWith(adapter)), isNull);
      expect(adapter.lastRequest, isNull);
    });
  });
}
