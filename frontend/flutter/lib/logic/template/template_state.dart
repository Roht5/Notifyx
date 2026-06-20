import 'package:equatable/equatable.dart';
import '../../data/models/template_model.dart';

abstract class TemplateState extends Equatable {
  const TemplateState();

  @override
  List<Object?> get props => [];
}

class TemplateInitial extends TemplateState {}

class TemplateLoading extends TemplateState {}

class TemplateLoaded extends TemplateState {
  final List<TemplateModel> templates;

  const TemplateLoaded(this.templates);

  @override
  List<Object?> get props => [templates];
}

class TemplateFailure extends TemplateState {
  final String error;

  const TemplateFailure(this.error);

  @override
  List<Object?> get props => [error];
}
