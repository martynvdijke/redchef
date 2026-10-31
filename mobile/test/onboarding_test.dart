import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:redchef/src/api/api_client.dart';
import 'package:redchef/src/auth/auth_provider.dart';
import 'package:redchef/src/screens/onboarding_screen.dart';

Future<void> pumpOnboarding(WidgetTester tester, ApiClient api) async {
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        apiClientProvider.overrideWithValue(api),
        // No network in widget tests; the first-run probe is a no-op here.
        serverUrlValidatorProvider.overrideWithValue((_) async => null),
      ],
      child: const MaterialApp(home: OnboardingScreen()),
    ),
  );
  // Let the persisted-URL lookup and first-run prefill settle.
  await tester.pump();
  await tester.pump();
}

void main() {
  testWidgets('first run prefills the hosted default server URL', (tester) async {
    final api = ApiClient();
    expect(api.baseUrl, isNull);

    await pumpOnboarding(tester, api);

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller?.text, defaultRedChefBaseUrl);
    // Prefilling is not persisting: the user still confirms the server.
    expect(api.baseUrl, isNull);

    expect(find.textContaining('Use the default instance below'), findsOneWidget);
    expect(find.textContaining('/api/setup/status'), findsOneWidget);
    expect(find.textContaining('redchef.example.com'), findsNothing);
  });

  testWidgets('a saved server URL takes precedence over the default', (tester) async {
    final api = ApiClient();
    api.setBaseUrlSync('https://chef.example.org');

    await pumpOnboarding(tester, api);

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller?.text, 'https://chef.example.org');
    expect(field.controller?.text, isNot(defaultRedChefBaseUrl));
    expect(api.baseUrl, 'https://chef.example.org');
  });

  testWidgets('empty input shows a validation error without a network call', (tester) async {
    final api = ApiClient();

    await pumpOnboarding(tester, api);
    await tester.enterText(find.byType(TextField), '');

    await tester.tap(find.text('Continue'));
    await tester.pump();

    expect(find.text('Enter your RedChef server URL'), findsOneWidget);
    expect(api.baseUrl, isNull);
  });
}
