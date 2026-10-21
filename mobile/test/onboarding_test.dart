import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:redchef/src/api/api_client.dart';
import 'package:redchef/src/auth/auth_provider.dart';
import 'package:redchef/src/screens/onboarding_screen.dart';

void main() {
  testWidgets('onboarding starts empty and has no hardcoded server address', (tester) async {
    final api = ApiClient();
    expect(api.baseUrl, isNull);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: OnboardingScreen()),
      ),
    );

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.controller?.text, isEmpty);

    expect(find.textContaining('10.0.2.2'), findsNothing);
    expect(find.textContaining('redchef.example.com'), findsNothing);
    expect(find.textContaining('emulator default'), findsNothing);
    expect(find.textContaining('Use emulator'), findsNothing);

    // The helper copy about validation is still present.
    expect(find.textContaining('/api/setup/status'), findsOneWidget);
  });

  testWidgets('empty input shows a validation error without a network call', (tester) async {
    final api = ApiClient();
    expect(api.baseUrl, isNull);

    await tester.pumpWidget(
      ProviderScope(
        overrides: [apiClientProvider.overrideWithValue(api)],
        child: const MaterialApp(home: OnboardingScreen()),
      ),
    );

    await tester.tap(find.text('Continue'));
    await tester.pump();

    expect(find.text('Enter your RedChef server URL'), findsOneWidget);
    expect(api.baseUrl, isNull);
  });
}
