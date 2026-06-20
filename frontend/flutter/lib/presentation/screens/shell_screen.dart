import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import '../../logic/tenant/tenant_bloc.dart';
import '../../logic/tenant/tenant_event.dart';
import '../../logic/tenant/tenant_state.dart';
import '../../logic/websocket/ws_bloc.dart';
import '../../logic/websocket/ws_event.dart';
import '../../core/theme/app_colors.dart';

class ShellScreen extends StatefulWidget {
  final Widget child;

  const ShellScreen({super.key, required this.child});

  @override
  State<ShellScreen> createState() => _ShellScreenState();
}

class _ShellScreenState extends State<ShellScreen> {
  bool _isSidebarCollapsed = false;

  int _getSelectedIndex(BuildContext context) {
    final String location = GoRouterState.of(context).matchedLocation;
    if (location.startsWith('/dashboard')) return 0;
    if (location.startsWith('/analytics')) return 1;
    if (location.startsWith('/send')) return 2;
    if (location.startsWith('/history')) return 3;
    if (location.startsWith('/templates')) return 4;
    if (location.startsWith('/tenants')) return 5;
    return 0;
  }

  void _onDestinationSelected(int index) {
    switch (index) {
      case 0:
        context.go('/dashboard');
        break;
      case 1:
        context.go('/analytics');
        break;
      case 2:
        context.go('/send');
        break;
      case 3:
        context.go('/history');
        break;
      case 4:
        context.go('/templates');
        break;
      case 5:
        context.go('/tenants');
        break;
    }
  }

  @override
  Widget build(BuildContext context) {
    final isDesktop = MediaQuery.of(context).size.width >= 1024;
    final showCollapsed = _isSidebarCollapsed || !isDesktop;

    return BlocListener<TenantBloc, TenantState>(
      listener: (context, state) {
        if (state is TenantLoaded && state.activeTenant != null) {
          context.read<WsBloc>().add(
            ConnectWs(
              tenantId: state.activeTenant!.id,
              userId: 'dashboard_admin',
              apiKey: state.activeTenant!.apiKeys.isNotEmpty
                  ? state.activeTenant!.apiKeys.first
                  : '',
            ),
          );
        }
      },
      child: Scaffold(
        body: Row(
          children: [
            // Sidebar / Navigation Rail
            NavigationRail(
              extended: !showCollapsed,
              backgroundColor: AppColors.surfaceObsidian,
              selectedIndex: _getSelectedIndex(context),
              onDestinationSelected: _onDestinationSelected,
              leading: Column(
                children: [
                  const SizedBox(height: 16),
                  IconButton(
                    icon: Icon(showCollapsed ? Icons.menu : Icons.menu_open),
                    color: AppColors.neonTeal,
                    onPressed: () {
                      setState(() {
                        _isSidebarCollapsed = !_isSidebarCollapsed;
                      });
                    },
                  ),
                  const SizedBox(height: 24),
                ],
              ),
              destinations: const [
                NavigationRailDestination(
                  icon: Icon(Icons.dashboard_outlined),
                  selectedIcon: Icon(
                    Icons.dashboard,
                    color: AppColors.neonTeal,
                  ),
                  label: Text('Overview'),
                ),
                NavigationRailDestination(
                  icon: Icon(Icons.bar_chart_outlined),
                  selectedIcon: Icon(
                    Icons.bar_chart,
                    color: AppColors.neonTeal,
                  ),
                  label: Text('Analytics'),
                ),
                NavigationRailDestination(
                  icon: Icon(Icons.send_outlined),
                  selectedIcon: Icon(Icons.send, color: AppColors.neonTeal),
                  label: Text('Send Notification'),
                ),
                NavigationRailDestination(
                  icon: Icon(Icons.history_outlined),
                  selectedIcon: Icon(Icons.history, color: AppColors.neonTeal),
                  label: Text('History'),
                ),
                NavigationRailDestination(
                  icon: Icon(Icons.layers_outlined),
                  selectedIcon: Icon(Icons.layers, color: AppColors.neonTeal),
                  label: Text('Templates'),
                ),
                NavigationRailDestination(
                  icon: Icon(Icons.business_outlined),
                  selectedIcon: Icon(Icons.business, color: AppColors.neonTeal),
                  label: Text('Tenants'),
                ),
              ],
            ),

            const VerticalDivider(
              thickness: 1,
              width: 1,
              color: AppColors.borderSubtle,
            ),

            // Main View Panel
            Expanded(
              child: Column(
                children: [
                  // Top header bar
                  Container(
                    height: 70,
                    padding: const EdgeInsets.symmetric(horizontal: 24),
                    decoration: const BoxDecoration(
                      color: AppColors.surfaceObsidian,
                      border: Border(
                        bottom: BorderSide(color: AppColors.borderSubtle),
                      ),
                    ),
                    child: Row(
                      children: [
                        const Text(
                          'NOTIFYX',
                          style: TextStyle(
                            fontSize: 20,
                            fontWeight: FontWeight.bold,
                            letterSpacing: 2,
                            color: AppColors.textPrimary,
                          ),
                        ),
                        const SizedBox(width: 8),
                        Container(
                          width: 8,
                          height: 8,
                          decoration: const BoxDecoration(
                            shape: BoxShape.circle,
                            color: AppColors.emeraldGreen,
                          ),
                        ),
                        const Spacer(),

                        // Active Tenant Selector Dropdown
                        BlocBuilder<TenantBloc, TenantState>(
                          builder: (context, state) {
                            if (state is TenantLoaded) {
                              return Container(
                                padding: const EdgeInsets.symmetric(
                                  horizontal: 16,
                                ),
                                decoration: BoxDecoration(
                                  color: AppColors.background,
                                  borderRadius: BorderRadius.circular(10),
                                  border: Border.all(
                                    color: AppColors.borderSubtle,
                                  ),
                                ),
                                child: DropdownButtonHideUnderline(
                                  child: DropdownButton<String>(
                                    value: state.activeTenant?.id,
                                    dropdownColor: AppColors.surfaceObsidian,
                                    icon: const Icon(
                                      Icons.keyboard_arrow_down,
                                      color: AppColors.neonTeal,
                                    ),
                                    style: const TextStyle(
                                      color: AppColors.textPrimary,
                                      fontWeight: FontWeight.w600,
                                    ),
                                    items: state.tenants.map((t) {
                                      return DropdownMenuItem<String>(
                                        value: t.id,
                                        child: Text(t.name),
                                      );
                                    }).toList(),
                                    onChanged: (value) {
                                      if (value != null) {
                                        final selected = state.tenants
                                            .firstWhere((t) => t.id == value);
                                        context.read<TenantBloc>().add(
                                          ChangeActiveTenant(selected),
                                        );
                                      }
                                    },
                                  ),
                                ),
                              );
                            }
                            return const SizedBox.shrink();
                          },
                        ),
                      ],
                    ),
                  ),

                  // Screen contents
                  Expanded(
                    child: Container(
                      color: AppColors.background,
                      child: widget.child,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ), // close Scaffold
    ); // close BlocListener
  }
}
