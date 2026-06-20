import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import '../screens/shell_screen.dart';

// Import basic placeholder screens (we'll implement them as clean stubs)
import '../screens/dashboard/dashboard_screen.dart';
import '../screens/analytics/analytics_screen.dart';
import '../screens/send/send_screen.dart';
import '../screens/history/history_screen.dart';
import '../screens/templates/templates_screen.dart';
import '../screens/tenants/tenants_screen.dart';

class AppRouter {
  static final GoRouter router = GoRouter(
    initialLocation: '/dashboard',
    routes: [
      ShellRoute(
        builder: (context, state, child) {
          return ShellScreen(child: child);
        },
        routes: [
          GoRoute(
            path: '/dashboard',
            builder: (context, state) => const DashboardScreen(),
          ),
          GoRoute(
            path: '/analytics',
            builder: (context, state) => const AnalyticsScreen(),
          ),
          GoRoute(
            path: '/send',
            builder: (context, state) => const SendScreen(),
          ),
          GoRoute(
            path: '/history',
            builder: (context, state) => const HistoryScreen(),
          ),
          GoRoute(
            path: '/templates',
            builder: (context, state) => const TemplatesScreen(),
          ),
          GoRoute(
            path: '/tenants',
            builder: (context, state) => const TenantsScreen(),
          ),
        ],
      ),
    ],
  );
}
