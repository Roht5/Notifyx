import 'package:flutter_bloc/flutter_bloc.dart';
import '../../core/network/api_client.dart';
import '../../data/repositories/tenant_repository.dart';
import '../../data/models/tenant_model.dart';
import 'tenant_event.dart';
import 'tenant_state.dart';

class TenantBloc extends Bloc<TenantEvent, TenantState> {
  final TenantRepository tenantRepository;
  final ApiClient apiClient;

  TenantBloc({required this.tenantRepository, required this.apiClient})
    : super(TenantInitial()) {
    on<LoadTenants>(_onLoadTenants);
    on<ChangeActiveTenant>(_onChangeActiveTenant);
    on<CreateNewTenant>(_onCreateNewTenant);
    on<UpdateTenantChannelsEvent>(_onUpdateTenantChannels);
    on<UpdateTenantLimitsEvent>(_onUpdateTenantLimits);
    on<RotateTenantApiKey>(_onRotateApiKey);
    on<RevokeTenantApiKey>(_onRevokeApiKey);
  }

  Future<void> _onLoadTenants(
    LoadTenants event,
    Emitter<TenantState> emit,
  ) async {
    emit(TenantLoading());
    try {
      final tenants = await tenantRepository.getTenants();
      final active = tenants.isNotEmpty ? tenants.first : null;
      if (active != null) {
        apiClient.updateTenantContext(
          active.id,
          active.apiKeys.isNotEmpty ? active.apiKeys.first : null,
        );
      }
      emit(TenantLoaded(tenants: tenants, activeTenant: active));
    } catch (e) {
      emit(TenantFailure(e.toString()));
    }
  }

  void _onChangeActiveTenant(
    ChangeActiveTenant event,
    Emitter<TenantState> emit,
  ) {
    if (state is TenantLoaded) {
      final currentState = state as TenantLoaded;
      final active = event.tenant;
      apiClient.updateTenantContext(
        active.id,
        active.apiKeys.isNotEmpty ? active.apiKeys.first : null,
      );
      emit(TenantLoaded(tenants: currentState.tenants, activeTenant: active));
    }
  }

  Future<void> _onCreateNewTenant(
    CreateNewTenant event,
    Emitter<TenantState> emit,
  ) async {
    if (state is TenantLoaded) {
      final currentState = state as TenantLoaded;
      try {
        final newTenant = await tenantRepository.createTenant(
          event.name,
          event.globalRateCap,
        );
        final updatedList = List<TenantModel>.from(currentState.tenants)
          ..add(newTenant);
        emit(
          TenantLoaded(
            tenants: updatedList,
            activeTenant: currentState.activeTenant ?? newTenant,
          ),
        );
      } catch (e) {
        emit(TenantFailure(e.toString()));
      }
    }
  }

  Future<void> _onUpdateTenantChannels(
    UpdateTenantChannelsEvent event,
    Emitter<TenantState> emit,
  ) async {
    if (state is TenantLoaded) {
      final currentState = state as TenantLoaded;
      try {
        final updatedTenant = await tenantRepository.updateTenantChannels(
          event.tenantId,
          event.channels,
        );
        final updatedList = currentState.tenants
            .map((t) => t.id == event.tenantId ? updatedTenant : t)
            .toList();
        final newActive = currentState.activeTenant?.id == event.tenantId
            ? updatedTenant
            : currentState.activeTenant;

        if (newActive != null && newActive.id == event.tenantId) {
          apiClient.updateTenantContext(
            newActive.id,
            newActive.apiKeys.isNotEmpty ? newActive.apiKeys.first : null,
          );
        }

        emit(TenantLoaded(tenants: updatedList, activeTenant: newActive));
      } catch (e) {
        emit(TenantFailure(e.toString()));
      }
    }
  }

  Future<void> _onUpdateTenantLimits(
    UpdateTenantLimitsEvent event,
    Emitter<TenantState> emit,
  ) async {
    if (state is TenantLoaded) {
      final currentState = state as TenantLoaded;
      try {
        final updatedTenant = await tenantRepository.updateTenantRateLimits(
          event.tenantId,
          event.limits,
        );
        final updatedList = currentState.tenants
            .map((t) => t.id == event.tenantId ? updatedTenant : t)
            .toList();
        final newActive = currentState.activeTenant?.id == event.tenantId
            ? updatedTenant
            : currentState.activeTenant;

        emit(TenantLoaded(tenants: updatedList, activeTenant: newActive));
      } catch (e) {
        emit(TenantFailure(e.toString()));
      }
    }
  }

  Future<void> _onRotateApiKey(
    RotateTenantApiKey event,
    Emitter<TenantState> emit,
  ) async {
    if (state is TenantLoaded) {
      final currentState = state as TenantLoaded;
      try {
        final newKey = await tenantRepository.rotateApiKey(event.tenantId);
        final updatedList = currentState.tenants.map((t) {
          if (t.id == event.tenantId) {
            return t.copyWith(
              apiKeys: List<String>.from(t.apiKeys)..add(newKey),
            );
          }
          return t;
        }).toList();

        final newActive = updatedList.firstWhere(
          (t) => t.id == currentState.activeTenant?.id,
        );
        if (newActive.id == event.tenantId) {
          apiClient.updateTenantContext(newActive.id, newKey);
        }

        emit(TenantLoaded(tenants: updatedList, activeTenant: newActive));
      } catch (e) {
        emit(TenantFailure(e.toString()));
      }
    }
  }

  Future<void> _onRevokeApiKey(
    RevokeTenantApiKey event,
    Emitter<TenantState> emit,
  ) async {
    if (state is TenantLoaded) {
      final currentState = state as TenantLoaded;
      try {
        await tenantRepository.revokeApiKey(event.tenantId, event.keyId);
        final updatedList = currentState.tenants.map((t) {
          if (t.id == event.tenantId) {
            return t.copyWith(
              apiKeys: List<String>.from(t.apiKeys)..remove(event.keyId),
            );
          }
          return t;
        }).toList();
        final newActive = currentState.activeTenant?.id == event.tenantId
            ? updatedList.firstWhere((t) => t.id == event.tenantId)
            : currentState.activeTenant;

        emit(TenantLoaded(tenants: updatedList, activeTenant: newActive));
      } catch (e) {
        emit(TenantFailure(e.toString()));
      }
    }
  }
}
