import '../../core/network/api_client.dart';
import '../models/template_model.dart';

abstract class TemplateRepository {
  Future<List<TemplateModel>> getTemplates(String tenantId);
  Future<TemplateModel> createTemplate(String tenantId, TemplateModel template);
  Future<TemplateModel> updateTemplate(String tenantId, TemplateModel template);
  Future<void> deleteTemplate(String tenantId, String templateId);
}

class HttpTemplateRepository implements TemplateRepository {
  final ApiClient apiClient;

  HttpTemplateRepository(this.apiClient);

  @override
  Future<List<TemplateModel>> getTemplates(String tenantId) async {
    final response = await apiClient.dio.get('/api/v1/templates');
    final Map<String, dynamic> body = response.data ?? {};
    final List<dynamic> data = body['templates'] ?? [];
    return data
        .map((e) => TemplateModel.fromJson(e as Map<String, dynamic>))
        .toList();
  }

  @override
  Future<TemplateModel> createTemplate(
    String tenantId,
    TemplateModel template,
  ) async {
    final response = await apiClient.dio.post(
      '/api/v1/templates',
      data: template.toJson(),
    );
    return TemplateModel.fromJson(response.data as Map<String, dynamic>);
  }

  @override
  Future<TemplateModel> updateTemplate(
    String tenantId,
    TemplateModel template,
  ) async {
    final response = await apiClient.dio.put(
      '/api/v1/templates/${template.id}',
      data: template.toJson(),
    );
    return TemplateModel.fromJson(response.data as Map<String, dynamic>);
  }

  @override
  Future<void> deleteTemplate(String tenantId, String templateId) async {
    await apiClient.dio.delete('/api/v1/templates/$templateId');
  }
}

class MockTemplateRepository implements TemplateRepository {
  final List<TemplateModel> _mockTemplates = [
    TemplateModel(
      id: 'tmpl-welcome-email',
      tenantId: 'tenant-acme-uuid',
      name: 'Welcome Email',
      channel: 'email',
      subject: 'Welcome to Acme Corp, {{name}}!',
      body:
          'Hi {{name}},\n\nThank you for signing up for Acme Corp. Your account is now active. Your registered username is {{username}}.\n\nBest Regards,\nThe Team',
      createdAt: DateTime.now().subtract(const Duration(days: 10)),
    ),
    TemplateModel(
      id: 'tmpl-order-shipped',
      tenantId: 'tenant-acme-uuid',
      name: 'Order Shipped SMS',
      channel: 'sms',
      body:
          'Hi {{name}}, your order #{{orderId}} has been shipped and is on its way!',
      createdAt: DateTime.now().subtract(const Duration(days: 5)),
    ),
    TemplateModel(
      id: 'tmpl-security-alert',
      tenantId: 'tenant-globex-uuid',
      name: 'Security Alert Push',
      channel: 'push',
      subject: 'Security Alert',
      body:
          'A new login was detected from a new browser session on your Globex profile.',
      createdAt: DateTime.now().subtract(const Duration(days: 2)),
    ),
  ];

  @override
  Future<List<TemplateModel>> getTemplates(String tenantId) async {
    await Future.delayed(const Duration(milliseconds: 500));
    return _mockTemplates.where((t) => t.tenantId == tenantId).toList();
  }

  @override
  Future<TemplateModel> createTemplate(
    String tenantId,
    TemplateModel template,
  ) async {
    await Future.delayed(const Duration(milliseconds: 700));
    final newTemplate = template.copyWith(
      id: 'tmpl-${template.name.toLowerCase().replaceAll(' ', '-')}',
      tenantId: tenantId,
      createdAt: DateTime.now(),
    );
    _mockTemplates.add(newTemplate);
    return newTemplate;
  }

  @override
  Future<TemplateModel> updateTemplate(
    String tenantId,
    TemplateModel template,
  ) async {
    await Future.delayed(const Duration(milliseconds: 600));
    final index = _mockTemplates.indexWhere((t) => t.id == template.id);
    if (index != -1) {
      _mockTemplates[index] = template;
      return template;
    }
    throw Exception('Template not found');
  }

  @override
  Future<void> deleteTemplate(String tenantId, String templateId) async {
    await Future.delayed(const Duration(milliseconds: 400));
    _mockTemplates.removeWhere((t) => t.id == templateId);
  }
}
