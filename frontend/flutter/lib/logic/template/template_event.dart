import 'package:equatable/equatable.dart';
import '../../data/models/template_model.dart';

abstract class TemplateEvent extends Equatable {
  const TemplateEvent();

  @override
  List<Object?> get props => [];
}

class LoadTemplates extends TemplateEvent {
  final String tenantId;

  const LoadTemplates(this.tenantId);

  @override
  List<Object?> get props => [tenantId];
}

class CreateTemplateEvent extends TemplateEvent {
  final String tenantId;
  final TemplateModel template;

  const CreateTemplateEvent({required this.tenantId, required this.template});

  @override
  List<Object?> get props => [tenantId, template];
}

class UpdateTemplateEvent extends TemplateEvent {
  final String tenantId;
  final TemplateModel template;

  const UpdateTemplateEvent({required this.tenantId, required this.template});

  @override
  List<Object?> get props => [tenantId, template];
}

class DeleteTemplateEvent extends TemplateEvent {
  final String tenantId;
  final String templateId;

  const DeleteTemplateEvent({required this.tenantId, required this.templateId});

  @override
  List<Object?> get props => [tenantId, templateId];
}
