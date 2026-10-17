import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import '../auth/auth_provider.dart';
import '../api/models.dart';
import '../screens/onboarding_screen.dart';
import '../screens/login_screen.dart';
import '../screens/register_screen.dart';
import '../screens/feed_screen.dart';
import '../screens/detail_screen.dart';
import '../screens/composer_screen.dart';
import '../screens/favourites_screen.dart';
import '../screens/forgot_password_screen.dart';
import '../screens/reset_password_screen.dart';
import '../screens/admin_posts_screen.dart';
import '../screens/tokens_screen.dart';
import '../screens/account_screen.dart';

final routerProvider = Provider<GoRouter>((ref) {
  final auth = ref.watch(authProvider);
  return GoRouter(
    initialLocation: '/',
    redirect: (ctx, state) {
      final baseUrl = ref.read(apiClientProvider).baseUrl;
      final loc = state.matchedLocation;
      final isOnboarding = loc == '/onboarding';
      if ((baseUrl == null || baseUrl.isEmpty) && !isOnboarding) return '/onboarding';
      if (auth.loading) return null;
      // allow guest browsing; no forced redirect to login
      return null;
    },
    routes: [
      GoRoute(path: '/onboarding', builder: (_, __) => const OnboardingScreen()),
      GoRoute(path: '/login', builder: (_, __) => const LoginScreen()),
      GoRoute(path: '/register', builder: (_, __) => const RegisterScreen()),
      GoRoute(path: '/forgot', builder: (_, __) => const ForgotPasswordScreen()),
      GoRoute(path: '/reset', builder: (_, s) {
        final token = s.uri.queryParameters['token'];
        return ResetPasswordScreen(initialToken: token);
      }),
      GoRoute(path: '/', builder: (_, __) => const FeedScreen()),
      GoRoute(path: '/favourites', builder: (_, __) => const FavouritesScreen()),
      GoRoute(path: '/post/:id', builder: (_, s) => DetailScreen(postId: int.parse(s.pathParameters['id']!))),
      GoRoute(path: '/admin/posts', builder: (_, __) => const AdminPostsScreen()),
      GoRoute(path: '/tokens', builder: (_, __) => const TokensScreen()),
      GoRoute(path: '/account', builder: (_, __) => const AccountScreen()),
      GoRoute(
        path: '/composer',
        builder: (_, s) {
          final extra = s.extra;
          return ComposerScreen(editPost: extra is Post ? extra : null);
        },
      ),
    ],
    errorBuilder: (_, s) => Scaffold(body: Center(child: Text(s.error.toString()))),
  );
});
