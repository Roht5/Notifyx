import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:fl_chart/fl_chart.dart';
import '../../../data/models/analytics_model.dart';
import '../../../logic/analytics/analytics_bloc.dart';
import '../../../logic/analytics/analytics_event.dart';
import '../../../logic/analytics/analytics_state.dart';
import '../../../logic/tenant/tenant_bloc.dart';
import '../../../logic/tenant/tenant_state.dart';
import '../../../logic/websocket/ws_bloc.dart';
import '../../../logic/websocket/ws_state.dart';
import '../../../core/theme/app_colors.dart';
import '../../widgets/glass_card.dart';
import '../analytics/analytics_screen.dart' show kChannelColors;

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  String? _loadedTenantId;

  void _loadAnalytics(BuildContext context, String tenantId) {
    _loadedTenantId = tenantId;
    context.read<AnalyticsBloc>().add(LoadAnalytics(tenantId));
  }

  @override
  Widget build(BuildContext context) {
    return BlocListener<TenantBloc, TenantState>(
      listener: (context, state) {
        if (state is TenantLoaded &&
            state.activeTenant != null &&
            state.activeTenant!.id != _loadedTenantId) {
          _loadAnalytics(context, state.activeTenant!.id);
        }
      },
      child: BlocBuilder<TenantBloc, TenantState>(
        builder: (context, tenantState) {
          if (tenantState is TenantLoaded &&
              tenantState.activeTenant != null &&
              _loadedTenantId != tenantState.activeTenant!.id) {
            _loadAnalytics(context, tenantState.activeTenant!.id);
          }
          return BlocBuilder<AnalyticsBloc, AnalyticsState>(
            builder: (context, state) {
              return _buildBody(state);
            },
          );
        },
      ),
    );
  }

  Widget _buildBody(AnalyticsState state) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (state is AnalyticsLoading || state is AnalyticsInitial)
              const Padding(
                padding: EdgeInsets.symmetric(vertical: 48),
                child: Center(
                  child: CircularProgressIndicator(color: AppColors.neonTeal),
                ),
              )
            else if (state is AnalyticsFailure)
              GlassCard(
                padding: const EdgeInsets.all(24),
                child: Text(
                  'Failed to load dashboard: ${state.error}',
                  style: const TextStyle(color: AppColors.coralRed),
                ),
              )
            else if (state is AnalyticsLoaded)
              ..._buildLoaded(state.analytics),
          ],
        ),
      ),
    );
  }

  List<Widget> _buildLoaded(AnalyticsModel analytics) {
    return [
      LayoutBuilder(
        builder: (context, constraints) {
          final isDesktop = constraints.maxWidth > 800;
          final crossAxisCount = isDesktop
              ? 4
              : (constraints.maxWidth > 500 ? 2 : 1);
          return GridView.count(
            crossAxisCount: crossAxisCount,
            crossAxisSpacing: 16,
            mainAxisSpacing: 16,
            shrinkWrap: true,
            physics: const NeverScrollableScrollPhysics(),
            childAspectRatio: isDesktop ? 2.2 : 2.5,
            children: [
              _buildKpiCard(
                'Total Sent',
                analytics.totalSent.toString(),
                Icons.check_circle_outline,
                AppColors.neonTeal,
              ),
              _buildKpiCard(
                'Delivered',
                analytics.totalDelivered.toString(),
                Icons.done_all,
                AppColors.emeraldGreen,
              ),
              _buildKpiCard(
                'Failed',
                analytics.totalFailed.toString(),
                Icons.error_outline,
                AppColors.coralRed,
              ),
              _buildKpiCard(
                'Delivery Rate',
                '${analytics.overallDeliveryRate.toStringAsFixed(1)}%',
                Icons.speed,
                AppColors.electricViolet,
              ),
            ],
          );
        },
      ),
      const SizedBox(height: 24),
      LayoutBuilder(
        builder: (context, constraints) {
          final isDesktop = constraints.maxWidth > 900;
          return Flex(
            direction: isDesktop ? Axis.horizontal : Axis.vertical,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                flex: isDesktop ? 2 : 0,
                child: _DlqTrendChartCard(dlqTrend: analytics.dlqTrend),
              ),
              if (isDesktop) const SizedBox(width: 24),
              if (!isDesktop) const SizedBox(height: 24),
              Expanded(
                flex: isDesktop ? 1 : 0,
                child: _ChannelDistributionCard(channels: analytics.channels),
              ),
            ],
          );
        },
      ),
      const SizedBox(height: 24),
      const _RecentActivityCard(),
    ];
  }

  Widget _buildKpiCard(String title, String value, IconData icon, Color color) {
    return GlassCard(
      padding: const EdgeInsets.all(20),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                title,
                style: const TextStyle(
                  color: AppColors.textSecondary,
                  fontSize: 14,
                ),
              ),
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: color.withOpacity(0.1),
                  borderRadius: BorderRadius.circular(8),
                ),
                child: Icon(icon, color: color, size: 20),
              ),
            ],
          ),
          Text(
            value,
            style: const TextStyle(
              color: AppColors.textPrimary,
              fontSize: 28,
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }
}

class _DlqTrendChartCard extends StatelessWidget {
  final List<DlqTrendPoint> dlqTrend;

  const _DlqTrendChartCard({required this.dlqTrend});

  @override
  Widget build(BuildContext context) {
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Dead-Letter Queue Trend',
            style: TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 24),
          SizedBox(
            height: 300,
            child: dlqTrend.isEmpty
                ? const Center(
                    child: Text(
                      'No DLQ activity',
                      style: TextStyle(color: AppColors.textSecondary),
                    ),
                  )
                : BarChart(
                    BarChartData(
                      alignment: BarChartAlignment.spaceAround,
                      barTouchData: BarTouchData(enabled: false),
                      gridData: FlGridData(
                        show: true,
                        drawVerticalLine: false,
                        getDrawingHorizontalLine: (value) => const FlLine(
                          color: AppColors.borderSubtle,
                          strokeWidth: 1,
                          dashArray: [5, 5],
                        ),
                      ),
                      titlesData: FlTitlesData(
                        leftTitles: const AxisTitles(
                          sideTitles: SideTitles(
                            showTitles: true,
                            reservedSize: 40,
                          ),
                        ),
                        bottomTitles: AxisTitles(
                          sideTitles: SideTitles(
                            showTitles: true,
                            getTitlesWidget: (value, _) {
                              final index = value.toInt();
                              if (index < 0 || index >= dlqTrend.length) {
                                return const SizedBox.shrink();
                              }
                              final date = dlqTrend[index].date;
                              return Padding(
                                padding: const EdgeInsets.only(top: 8),
                                child: Text(
                                  '${date.month}/${date.day}',
                                  style: const TextStyle(
                                    color: AppColors.textSecondary,
                                    fontSize: 12,
                                  ),
                                ),
                              );
                            },
                          ),
                        ),
                        rightTitles: const AxisTitles(
                          sideTitles: SideTitles(showTitles: false),
                        ),
                        topTitles: const AxisTitles(
                          sideTitles: SideTitles(showTitles: false),
                        ),
                      ),
                      borderData: FlBorderData(show: false),
                      barGroups: [
                        for (int i = 0; i < dlqTrend.length; i++)
                          BarChartGroupData(
                            x: i,
                            barRods: [
                              BarChartRodData(
                                toY: dlqTrend[i].count.toDouble(),
                                color: AppColors.coralRed,
                                width: 12,
                                borderRadius: const BorderRadius.only(
                                  topLeft: Radius.circular(4),
                                  topRight: Radius.circular(4),
                                ),
                              ),
                            ],
                          ),
                      ],
                    ),
                  ),
          ),
        ],
      ),
    );
  }
}

class _ChannelDistributionCard extends StatelessWidget {
  final List<ChannelBreakdown> channels;

  const _ChannelDistributionCard({required this.channels});

  @override
  Widget build(BuildContext context) {
    final total = channels.fold<int>(0, (a, c) => a + c.sent);
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Channel Distribution',
            style: TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 24),
          if (total == 0)
            const Text(
              'No data yet',
              style: TextStyle(color: AppColors.textSecondary),
            )
          else
            ...channels.map(
              (c) => Padding(
                padding: const EdgeInsets.only(bottom: 16),
                child: _buildDistRow(
                  c.channel,
                  ((c.sent / total) * 100).round(),
                  kChannelColors[c.channel] ?? AppColors.textMuted,
                ),
              ),
            ),
        ],
      ),
    );
  }

  Widget _buildDistRow(String label, int value, Color color) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(
              label,
              style: const TextStyle(
                color: AppColors.textPrimary,
                fontSize: 14,
              ),
            ),
            Text(
              '$value%',
              style: TextStyle(
                color: color,
                fontSize: 14,
                fontWeight: FontWeight.bold,
              ),
            ),
          ],
        ),
        const SizedBox(height: 8),
        Container(
          height: 8,
          width: double.infinity,
          decoration: BoxDecoration(
            color: AppColors.surfaceObsidian,
            borderRadius: BorderRadius.circular(4),
          ),
          child: FractionallySizedBox(
            alignment: Alignment.centerLeft,
            widthFactor: value / 100,
            child: Container(
              decoration: BoxDecoration(
                color: color,
                borderRadius: BorderRadius.circular(4),
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _RecentActivityCard extends StatelessWidget {
  const _RecentActivityCard();

  @override
  Widget build(BuildContext context) {
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Recent Activity',
            style: TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          BlocBuilder<WsBloc, WsState>(
            builder: (context, state) {
              if (state is WsConnected && state.eventsLog.isNotEmpty) {
                return ListView.builder(
                  shrinkWrap: true,
                  physics: const NeverScrollableScrollPhysics(),
                  itemCount: state.eventsLog.length > 5
                      ? 5
                      : state.eventsLog.length,
                  itemBuilder: (context, index) {
                    final rawEvent = state.eventsLog[index];
                    Map<String, dynamic> event = {};
                    try {
                      event = jsonDecode(rawEvent) as Map<String, dynamic>;
                    } catch (_) {}
                    final isError = event['status'] == 'failed';
                    return Container(
                      margin: const EdgeInsets.only(bottom: 12),
                      padding: const EdgeInsets.all(16),
                      decoration: BoxDecoration(
                        color: AppColors.surfaceObsidian.withOpacity(0.5),
                        borderRadius: BorderRadius.circular(12),
                        border: Border.all(
                          color: AppColors.borderSubtle.withOpacity(0.5),
                        ),
                      ),
                      child: Row(
                        children: [
                          Container(
                            padding: const EdgeInsets.all(10),
                            decoration: BoxDecoration(
                              color: isError
                                  ? AppColors.coralRed.withOpacity(0.1)
                                  : AppColors.emeraldGreen.withOpacity(0.1),
                              borderRadius: BorderRadius.circular(10),
                            ),
                            child: Icon(
                              isError
                                  ? Icons.error_outline
                                  : Icons.check_circle_outline,
                              color: isError
                                  ? AppColors.coralRed
                                  : AppColors.emeraldGreen,
                            ),
                          ),
                          const SizedBox(width: 16),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  event['template'] ?? event['id'] ?? 'System Event',
                                  style: const TextStyle(
                                    color: AppColors.textPrimary,
                                    fontWeight: FontWeight.w600,
                                  ),
                                ),
                                Text(
                                  'ID: ${event['notificationId']?.toString().substring(0, 8) ?? 'N/A'}',
                                  style: const TextStyle(
                                    color: AppColors.textSecondary,
                                    fontSize: 12,
                                  ),
                                ),
                              ],
                            ),
                          ),
                          Text(
                            event['status']?.toString().toUpperCase() ?? 'UNKNOWN',
                            style: TextStyle(
                              color: isError
                                  ? AppColors.coralRed
                                  : AppColors.emeraldGreen,
                              fontWeight: FontWeight.bold,
                              fontSize: 12,
                            ),
                          ),
                        ],
                      ),
                    );
                  },
                );
              }
              return const Padding(
                padding: EdgeInsets.symmetric(vertical: 16),
                child: Text(
                  'No recent activity yet',
                  style: TextStyle(color: AppColors.textSecondary),
                ),
              );
            },
          ),
        ],
      ),
    );
  }
}
