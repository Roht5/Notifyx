import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import '../../../data/models/notification_model.dart';
import '../../../logic/notification/notification_bloc.dart';
import '../../../logic/notification/notification_event.dart';
import '../../../logic/notification/notification_state.dart';
import '../../../logic/tenant/tenant_bloc.dart';
import '../../../logic/tenant/tenant_state.dart';
import '../../../core/theme/app_colors.dart';
import '../../widgets/glass_card.dart';

class HistoryScreen extends StatefulWidget {
  const HistoryScreen({super.key});

  @override
  State<HistoryScreen> createState() => _HistoryScreenState();
}

class _HistoryScreenState extends State<HistoryScreen> {
  String searchQuery = '';
  bool showFilters = false;
  String? selectedId;
  String? _loadedTenantId;

  void _loadHistory(BuildContext context, String tenantId) {
    _loadedTenantId = tenantId;
    context.read<NotificationBloc>().add(
      LoadNotificationHistory(tenantId: tenantId, refresh: true),
    );
  }

  @override
  Widget build(BuildContext context) {
    return BlocListener<TenantBloc, TenantState>(
      listener: (context, state) {
        if (state is TenantLoaded &&
            state.activeTenant != null &&
            state.activeTenant!.id != _loadedTenantId) {
          _loadHistory(context, state.activeTenant!.id);
        }
      },
      child: BlocBuilder<TenantBloc, TenantState>(
        builder: (context, tenantState) {
          if (tenantState is TenantLoaded &&
              tenantState.activeTenant != null &&
              _loadedTenantId != tenantState.activeTenant!.id) {
            _loadHistory(context, tenantState.activeTenant!.id);
          }
          return BlocBuilder<NotificationBloc, NotificationState>(
            builder: (context, state) {
              return _buildBody(context, state);
            },
          );
        },
      ),
    );
  }

  Widget _buildBody(BuildContext context, NotificationState state) {
    List<NotificationModel> history = [];
    bool isLoading = false;
    String? error;

    if (state is NotificationHistoryLoaded) {
      history = state.history;
    } else if (state is NotificationHistoryLoading) {
      isLoading = true;
    } else if (state is NotificationHistoryFailure) {
      error = state.error;
    }

    final filteredHistory = history
        .where(
          (record) =>
              record.recipientId.toLowerCase().contains(
                searchQuery.toLowerCase(),
              ) ||
              record.channel.toLowerCase().contains(
                searchQuery.toLowerCase(),
              ) ||
              (record.recipientEmail ?? '').toLowerCase().contains(
                searchQuery.toLowerCase(),
              ),
        )
        .toList();

    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // Controls
            Wrap(
              spacing: 16,
              runSpacing: 16,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Container(
                  width: 300,
                  padding: const EdgeInsets.symmetric(horizontal: 16),
                  decoration: BoxDecoration(
                    color: AppColors.surfaceCard,
                    border: Border.all(color: AppColors.borderSubtle),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: Row(
                    children: [
                      const Icon(
                        Icons.search,
                        color: AppColors.textMuted,
                        size: 20,
                      ),
                      const SizedBox(width: 8),
                      Expanded(
                        child: TextField(
                          onChanged: (val) => setState(() => searchQuery = val),
                          style: const TextStyle(color: AppColors.textPrimary),
                          decoration: const InputDecoration(
                            border: InputBorder.none,
                            hintText: 'Search notifications...',
                            hintStyle: TextStyle(color: AppColors.textMuted),
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                GestureDetector(
                  onTap: () => setState(() => showFilters = !showFilters),
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 12,
                    ),
                    decoration: BoxDecoration(
                      color: showFilters
                          ? AppColors.neonTeal.withOpacity(0.2)
                          : AppColors.surfaceObsidian.withOpacity(0.5),
                      border: Border.all(
                        color: showFilters
                            ? AppColors.neonTeal
                            : AppColors.borderSubtle,
                      ),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.filter_list,
                          size: 18,
                          color: showFilters
                              ? AppColors.neonTeal
                              : AppColors.textPrimary,
                        ),
                        const SizedBox(width: 8),
                        Text(
                          'Filter',
                          style: TextStyle(
                            color: showFilters
                                ? AppColors.neonTeal
                                : AppColors.textPrimary,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                GestureDetector(
                  onTap: () {
                    final tenantState = context.read<TenantBloc>().state;
                    if (tenantState is TenantLoaded &&
                        tenantState.activeTenant != null) {
                      _loadHistory(context, tenantState.activeTenant!.id);
                    }
                  },
                  child: Container(
                    padding: const EdgeInsets.symmetric(
                      horizontal: 16,
                      vertical: 12,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.surfaceObsidian.withOpacity(0.5),
                      border: Border.all(color: AppColors.borderSubtle),
                      borderRadius: BorderRadius.circular(8),
                    ),
                    child: const Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Icon(
                          Icons.refresh,
                          size: 18,
                          color: AppColors.textPrimary,
                        ),
                        SizedBox(width: 8),
                        Text(
                          'Refresh',
                          style: TextStyle(
                            color: AppColors.textPrimary,
                            fontWeight: FontWeight.w500,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 24),

            // Filter Panel
            if (showFilters) ...[
              GlassCard(
                padding: const EdgeInsets.all(16),
                child: Wrap(
                  spacing: 16,
                  runSpacing: 16,
                  children: [
                    _buildFilterDropdown('Status', [
                      'All Status',
                      'delivered',
                      'queued',
                      'failed',
                      'retrying',
                    ]),
                    _buildFilterDropdown('Channel', [
                      'All Channels',
                      'email',
                      'sms',
                      'push',
                      'websocket',
                    ]),
                  ],
                ),
              ),
              const SizedBox(height: 24),
            ],

            if (isLoading)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 48),
                child: Center(
                  child: CircularProgressIndicator(color: AppColors.neonTeal),
                ),
              )
            else if (error != null)
              GlassCard(
                padding: const EdgeInsets.all(24),
                child: Text(
                  'Failed to load history: $error',
                  style: const TextStyle(color: AppColors.coralRed),
                ),
              )
            else
              // Table
              GlassCard(
                padding: const EdgeInsets.all(0),
                child: SingleChildScrollView(
                  scrollDirection: Axis.horizontal,
                  child: DataTable(
                    showCheckboxColumn: false,
                    columnSpacing: 32,
                    headingRowColor: MaterialStateProperty.all(
                      AppColors.surfaceObsidian,
                    ),
                    headingTextStyle: const TextStyle(
                      color: AppColors.textMuted,
                      fontWeight: FontWeight.w600,
                    ),
                    dataRowColor: MaterialStateProperty.resolveWith<Color?>((
                      Set<MaterialState> states,
                    ) {
                      if (states.contains(MaterialState.hovered)) {
                        return AppColors.surfaceObsidian.withOpacity(0.8);
                      }
                      return null;
                    }),
                    columns: const [
                      DataColumn(label: Text('Recipient')),
                      DataColumn(label: Text('Channel')),
                      DataColumn(label: Text('Priority')),
                      DataColumn(label: Text('Attempts'), numeric: true),
                      DataColumn(label: Text('Status')),
                      DataColumn(label: Text('Date')),
                      DataColumn(label: Text('')),
                    ],
                    rows: filteredHistory.map((record) {
                      final status = record.status;
                      Color statusColor = AppColors.neonTeal;
                      if (status == 'delivered') {
                        statusColor = AppColors.emeraldGreen;
                      }
                      if (status == 'failed') statusColor = AppColors.coralRed;

                      final isSelected = selectedId == record.id;
                      final recipient =
                          record.recipientEmail ??
                          record.recipientPhone ??
                          record.recipientToken ??
                          record.recipientId;

                      return DataRow(
                        selected: isSelected,
                        onSelectChanged: (_) {
                          setState(() {
                            selectedId = isSelected ? null : record.id;
                          });
                        },
                        cells: [
                          DataCell(
                            Text(
                              recipient,
                              style: const TextStyle(
                                color: AppColors.textPrimary,
                                fontWeight: FontWeight.w500,
                              ),
                            ),
                          ),
                          DataCell(
                            Text(
                              record.channel,
                              style: const TextStyle(
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                          DataCell(
                            Text(
                              record.priority,
                              style: const TextStyle(
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                          DataCell(
                            Text(
                              record.attempts.length.toString(),
                              style: const TextStyle(
                                color: AppColors.textPrimary,
                                fontWeight: FontWeight.bold,
                              ),
                            ),
                          ),
                          DataCell(
                            Container(
                              padding: const EdgeInsets.symmetric(
                                horizontal: 12,
                                vertical: 4,
                              ),
                              decoration: BoxDecoration(
                                color: statusColor.withOpacity(0.1),
                                borderRadius: BorderRadius.circular(16),
                              ),
                              child: Text(
                                status,
                                style: TextStyle(
                                  color: statusColor,
                                  fontSize: 12,
                                  fontWeight: FontWeight.bold,
                                ),
                              ),
                            ),
                          ),
                          DataCell(
                            Text(
                              record.createdAt.toLocal().toString().split(
                                '.',
                              )[0],
                              style: const TextStyle(
                                color: AppColors.textSecondary,
                              ),
                            ),
                          ),
                          const DataCell(
                            Icon(
                              Icons.chevron_right,
                              color: AppColors.textMuted,
                              size: 20,
                            ),
                          ),
                        ],
                      );
                    }).toList(),
                  ),
                ),
              ),

            // Detail View
            if (selectedId != null) ...[
              const SizedBox(height: 24),
              _buildDetail(
                filteredHistory.firstWhere((n) => n.id == selectedId),
              ),
            ],
          ],
        ),
      ),
    );
  }

  Widget _buildDetail(NotificationModel record) {
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Notification Details',
            style: TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          Text(
            record.body,
            style: const TextStyle(color: AppColors.textSecondary),
          ),
          const SizedBox(height: 16),
          LayoutBuilder(
            builder: (context, constraints) {
              return Wrap(
                spacing: 16,
                runSpacing: 16,
                children: [
                  _buildDetailBox('Status', record.status, constraints.maxWidth),
                  _buildDetailBox(
                    'Attempts',
                    record.attempts.length.toString(),
                    constraints.maxWidth,
                  ),
                  _buildDetailBox(
                    'Last Error',
                    record.attempts.isNotEmpty
                        ? (record.attempts.last.errorMessage ?? 'None')
                        : 'None',
                    constraints.maxWidth,
                  ),
                  _buildDetailBox(
                    'Template',
                    record.templateId ?? 'None',
                    constraints.maxWidth,
                  ),
                ],
              );
            },
          ),
        ],
      ),
    );
  }

  Widget _buildFilterDropdown(String label, List<String> items) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          label,
          style: const TextStyle(
            color: AppColors.textPrimary,
            fontSize: 14,
            fontWeight: FontWeight.w500,
          ),
        ),
        const SizedBox(height: 8),
        Container(
          width: 200,
          padding: const EdgeInsets.symmetric(horizontal: 12),
          decoration: BoxDecoration(
            color: AppColors.surfaceCard,
            border: Border.all(color: AppColors.borderSubtle),
            borderRadius: BorderRadius.circular(8),
          ),
          child: DropdownButtonHideUnderline(
            child: DropdownButton<String>(
              value: items.first,
              dropdownColor: AppColors.surfaceObsidian,
              isExpanded: true,
              style: const TextStyle(color: AppColors.textPrimary),
              icon: const Icon(
                Icons.arrow_drop_down,
                color: AppColors.textMuted,
              ),
              items: items
                  .map((e) => DropdownMenuItem(value: e, child: Text(e)))
                  .toList(),
              onChanged: (val) {
                if (val == null) return;
                final tenantState = context.read<TenantBloc>().state;
                if (tenantState is! TenantLoaded ||
                    tenantState.activeTenant == null) {
                  return;
                }
                final tenantId = tenantState.activeTenant!.id;
                final isStatus = items.contains('All Status');
                context.read<NotificationBloc>().add(
                  LoadNotificationHistory(
                    tenantId: tenantId,
                    status: isStatus && val != 'All Status' ? val : null,
                    channel: !isStatus && val != 'All Channels' ? val : null,
                    refresh: true,
                  ),
                );
              },
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildDetailBox(String label, String value, double parentWidth) {
    final width = parentWidth > 600
        ? (parentWidth - 48) / 4
        : (parentWidth - 16) / 2;
    return Container(
      width: width,
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.surfaceObsidian.withOpacity(0.5),
        border: Border.all(color: AppColors.borderSubtle.withOpacity(0.5)),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: const TextStyle(
              color: AppColors.textSecondary,
              fontSize: 12,
            ),
          ),
          const SizedBox(height: 4),
          Text(
            value,
            style: const TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }
}
