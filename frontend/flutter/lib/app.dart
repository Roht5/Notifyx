import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'core/network/api_client.dart';
import 'core/theme/app_theme.dart';
import 'data/repositories/tenant_repository.dart';
import 'data/repositories/template_repository.dart';
import 'data/repositories/notification_repository.dart';
import 'data/repositories/analytics_repository.dart';
import 'logic/tenant/tenant_bloc.dart';
import 'logic/tenant/tenant_event.dart';
import 'logic/template/template_bloc.dart';
import 'logic/notification/notification_bloc.dart';
import 'logic/analytics/analytics_bloc.dart';
import 'presentation/routing/app_router.dart';

import 'core/network/websocket_client.dart';
import 'logic/websocket/ws_bloc.dart';

class App extends StatelessWidget {
  final ApiClient apiClient;
  final WebSocketClient webSocketClient;
  final TenantRepository tenantRepository;
  final TemplateRepository templateRepository;
  final NotificationRepository notificationRepository;
  final AnalyticsRepository analyticsRepository;
  final bool useMockRepositories;

  const App({
    super.key,
    required this.apiClient,
    required this.webSocketClient,
    required this.tenantRepository,
    required this.templateRepository,
    required this.notificationRepository,
    required this.analyticsRepository,
    required this.useMockRepositories,
  });

  @override
  Widget build(BuildContext context) {
    return MultiRepositoryProvider(
      providers: [
        RepositoryProvider<ApiClient>.value(value: apiClient),
        RepositoryProvider<WebSocketClient>.value(value: webSocketClient),
        RepositoryProvider<TenantRepository>.value(value: tenantRepository),
        RepositoryProvider<TemplateRepository>.value(value: templateRepository),
        RepositoryProvider<NotificationRepository>.value(
          value: notificationRepository,
        ),
        RepositoryProvider<AnalyticsRepository>.value(
          value: analyticsRepository,
        ),
      ],
      child: MultiBlocProvider(
        providers: [
          BlocProvider<TenantBloc>(
            create: (context) => TenantBloc(
              tenantRepository: tenantRepository,
              apiClient: apiClient,
            )..add(LoadTenants()),
          ),
          BlocProvider<WsBloc>(
            create: (context) => WsBloc(
              webSocketClient: webSocketClient,
              useMock: useMockRepositories,
            ),
          ),
          BlocProvider<TemplateBloc>(
            create: (context) =>
                TemplateBloc(templateRepository: templateRepository),
          ),
          BlocProvider<NotificationBloc>(
            create: (context) => NotificationBloc(
              notificationRepository: notificationRepository,
            ),
          ),
          BlocProvider<AnalyticsBloc>(
            create: (context) =>
                AnalyticsBloc(analyticsRepository: analyticsRepository),
          ),
        ],
        child: MaterialApp.router(
          title: 'Notifyx Dashboard',
          debugShowCheckedModeBanner: false,
          theme: AppTheme.darkTheme,
          routerConfig: AppRouter.router,
        ),
      ),
    );
  }
}
