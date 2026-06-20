import 'package:flutter/material.dart';
import 'app.dart';
import 'core/network/api_client.dart';
import 'data/repositories/tenant_repository.dart';
import 'data/repositories/template_repository.dart';
import 'data/repositories/notification_repository.dart';
import 'data/repositories/analytics_repository.dart';

import 'core/network/websocket_client.dart';

void main() {
  WidgetsFlutterBinding.ensureInitialized();

  // Toggle Mock Mode: Set to false to connect to the real Go backend API
  const bool useMockRepositories = false;

  // Base URL config (can load from Environment variables in production)
  const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:8081',
  );

  const String wsBaseUrl = String.fromEnvironment(
    'WS_BASE_URL',
    defaultValue: 'ws://localhost:8081',
  );

  final apiClient = ApiClient(baseUrl: apiBaseUrl);
  final webSocketClient = WebSocketClient(wsBaseUrl: wsBaseUrl);

  // Instantiating repositories
  final TenantRepository tenantRepository;
  final TemplateRepository templateRepository;
  final NotificationRepository notificationRepository;
  final AnalyticsRepository analyticsRepository;

  if (useMockRepositories) {
    tenantRepository = MockTenantRepository();
    templateRepository = MockTemplateRepository();
    notificationRepository = MockNotificationRepository();
    analyticsRepository = MockAnalyticsRepository();
  } else {
    tenantRepository = HttpTenantRepository(apiClient);
    templateRepository = HttpTemplateRepository(apiClient);
    notificationRepository = HttpNotificationRepository(apiClient);
    analyticsRepository = HttpAnalyticsRepository(apiClient);
  }

  runApp(
    App(
      apiClient: apiClient,
      webSocketClient: webSocketClient,
      tenantRepository: tenantRepository,
      templateRepository: templateRepository,
      notificationRepository: notificationRepository,
      analyticsRepository: analyticsRepository,
      useMockRepositories: useMockRepositories,
    ),
  );
}
