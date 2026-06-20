import 'package:equatable/equatable.dart';
import '../../data/models/tenant_model.dart';

abstract class TenantEvent extends Equatable {
  const TenantEvent();

  @override
  List<Object?> get props => [];
}

class LoadTenants extends TenantEvent {}

class ChangeActiveTenant extends TenantEvent {
  final TenantModel tenant;

  const ChangeActiveTenant(this.tenant);

  @override
  List<Object?> get props => [tenant];
}

class CreateNewTenant extends TenantEvent {
  final String name;
  final int globalRateCap;

  const CreateNewTenant({required this.name, required this.globalRateCap});

  @override
  List<Object?> get props => [name, globalRateCap];
}

class UpdateTenantChannelsEvent extends TenantEvent {
  final String tenantId;
  final List<String> channels;

  const UpdateTenantChannelsEvent({
    required this.tenantId,
    required this.channels,
  });

  @override
  List<Object?> get props => [tenantId, channels];
}

class UpdateTenantLimitsEvent extends TenantEvent {
  final String tenantId;
  final Map<String, int> limits;

  const UpdateTenantLimitsEvent({required this.tenantId, required this.limits});

  @override
  List<Object?> get props => [tenantId, limits];
}

class RotateTenantApiKey extends TenantEvent {
  final String tenantId;

  const RotateTenantApiKey(this.tenantId);

  @override
  List<Object?> get props => [tenantId];
}

class RevokeTenantApiKey extends TenantEvent {
  final String tenantId;
  final String keyId;

  const RevokeTenantApiKey({required this.tenantId, required this.keyId});

  @override
  List<Object?> get props => [tenantId, keyId];
}
