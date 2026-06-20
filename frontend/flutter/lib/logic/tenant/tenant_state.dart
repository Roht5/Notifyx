import 'package:equatable/equatable.dart';
import '../../data/models/tenant_model.dart';

abstract class TenantState extends Equatable {
  const TenantState();

  @override
  List<Object?> get props => [];
}

class TenantInitial extends TenantState {}

class TenantLoading extends TenantState {}

class TenantLoaded extends TenantState {
  final List<TenantModel> tenants;
  final TenantModel? activeTenant;

  const TenantLoaded({required this.tenants, this.activeTenant});

  @override
  List<Object?> get props => [tenants, activeTenant];
}

class TenantFailure extends TenantState {
  final String error;

  const TenantFailure(this.error);

  @override
  List<Object?> get props => [error];
}
