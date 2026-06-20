import 'package:flutter_bloc/flutter_bloc.dart';
import '../../data/repositories/template_repository.dart';
import 'template_event.dart';
import 'template_state.dart';

class TemplateBloc extends Bloc<TemplateEvent, TemplateState> {
  final TemplateRepository templateRepository;

  TemplateBloc({required this.templateRepository}) : super(TemplateInitial()) {
    on<LoadTemplates>(_onLoadTemplates);
    on<CreateTemplateEvent>(_onCreateTemplate);
    on<UpdateTemplateEvent>(_onUpdateTemplate);
    on<DeleteTemplateEvent>(_onDeleteTemplate);
  }

  Future<void> _onLoadTemplates(
    LoadTemplates event,
    Emitter<TemplateState> emit,
  ) async {
    emit(TemplateLoading());
    try {
      final templates = await templateRepository.getTemplates(event.tenantId);
      emit(TemplateLoaded(templates));
    } catch (e) {
      emit(TemplateFailure(e.toString()));
    }
  }

  Future<void> _onCreateTemplate(
    CreateTemplateEvent event,
    Emitter<TemplateState> emit,
  ) async {
    if (state is TemplateLoaded) {
      final currentState = state as TemplateLoaded;
      try {
        final created = await templateRepository.createTemplate(
          event.tenantId,
          event.template,
        );
        emit(TemplateLoaded(List.from(currentState.templates)..add(created)));
      } catch (e) {
        emit(TemplateFailure(e.toString()));
      }
    }
  }

  Future<void> _onUpdateTemplate(
    UpdateTemplateEvent event,
    Emitter<TemplateState> emit,
  ) async {
    if (state is TemplateLoaded) {
      final currentState = state as TemplateLoaded;
      try {
        final updated = await templateRepository.updateTemplate(
          event.tenantId,
          event.template,
        );
        final list = currentState.templates
            .map((t) => t.id == updated.id ? updated : t)
            .toList();
        emit(TemplateLoaded(list));
      } catch (e) {
        emit(TemplateFailure(e.toString()));
      }
    }
  }

  Future<void> _onDeleteTemplate(
    DeleteTemplateEvent event,
    Emitter<TemplateState> emit,
  ) async {
    if (state is TemplateLoaded) {
      final currentState = state as TemplateLoaded;
      try {
        await templateRepository.deleteTemplate(
          event.tenantId,
          event.templateId,
        );
        final list = currentState.templates
            .where((t) => t.id != event.templateId)
            .toList();
        emit(TemplateLoaded(list));
      } catch (e) {
        emit(TemplateFailure(e.toString()));
      }
    }
  }
}
