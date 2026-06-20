import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:fl_chart/fl_chart.dart';
import '../../../data/models/analytics_model.dart';
import '../../../logic/analytics/analytics_bloc.dart';
import '../../../logic/analytics/analytics_event.dart';
import '../../../logic/analytics/analytics_state.dart';
import '../../../logic/tenant/tenant_bloc.dart';
import '../../../logic/tenant/tenant_state.dart';
import '../../../core/theme/app_colors.dart';
import '../../widgets/glass_card.dart';

const Map<String, Color> kChannelColors = {
  'email': AppColors.neonTeal,
  'sms': AppColors.electricViolet,
  'push': AppColors.emeraldGreen,
  'websocket': Colors.orange,
};

class AnalyticsScreen extends StatefulWidget {
  const AnalyticsScreen({super.key});

  @override
  State<AnalyticsScreen> createState() => _AnalyticsScreenState();
}

class _AnalyticsScreenState extends State<AnalyticsScreen> {
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
              return _buildBody(context, tenantState, state);
            },
          );
        },
      ),
    );
  }

  Widget _buildBody(
    BuildContext context,
    TenantState tenantState,
    AnalyticsState state,
  ) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(24.0),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                const Text(
                  'Analytics',
                  style: TextStyle(
                    color: AppColors.textPrimary,
                    fontSize: 24,
                    fontWeight: FontWeight.bold,
                  ),
                ),
                GestureDetector(
                  onTap: () {
                    if (tenantState is TenantLoaded &&
                        tenantState.activeTenant != null) {
                      _loadAnalytics(context, tenantState.activeTenant!.id);
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
                  'Failed to load analytics: ${state.error}',
                  style: const TextStyle(color: AppColors.coralRed),
                ),
              )
            else if (state is AnalyticsLoaded)
              _buildAnalyticsContent(state.analytics),
          ],
        ),
      ),
    );
  }

  Widget _buildAnalyticsContent(AnalyticsModel analytics) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        LayoutBuilder(
          builder: (context, constraints) {
            final isDesktop = constraints.maxWidth > 700;
            final cards = [
              _buildStatCard(
                'Total Sent',
                analytics.totalSent.toString(),
                AppColors.textPrimary,
              ),
              _buildStatCard(
                'Delivered',
                analytics.totalDelivered.toString(),
                AppColors.emeraldGreen,
              ),
              _buildStatCard(
                'Failed',
                analytics.totalFailed.toString(),
                AppColors.coralRed,
              ),
              _buildStatCard(
                'Delivery Rate',
                '${analytics.overallDeliveryRate.toStringAsFixed(1)}%',
                AppColors.neonTeal,
              ),
            ];
            return Flex(
              direction: isDesktop ? Axis.horizontal : Axis.vertical,
              children: [
                for (int i = 0; i < cards.length; i++) ...[
                  Expanded(flex: isDesktop ? 1 : 0, child: cards[i]),
                  if (i != cards.length - 1)
                    isDesktop
                        ? const SizedBox(width: 16)
                        : const SizedBox(height: 16),
                ],
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
                  child: _DlqTrendCard(dlqTrend: analytics.dlqTrend),
                ),
                if (isDesktop) const SizedBox(width: 24),
                if (!isDesktop) const SizedBox(height: 24),
                Expanded(
                  flex: isDesktop ? 1 : 0,
                  child: _ChannelPieCard(channels: analytics.channels),
                ),
              ],
            );
          },
        ),
        const SizedBox(height: 24),
        _ChannelDeliveryRatesCard(channels: analytics.channels),
      ],
    );
  }

  Widget _buildStatCard(String label, String value, Color color) {
    return GlassCard(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            label,
            style: const TextStyle(
              color: AppColors.textSecondary,
              fontSize: 14,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            value,
            style: TextStyle(
              color: color,
              fontSize: 24,
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }
}

class _ChannelPieCard extends StatelessWidget {
  final List<ChannelBreakdown> channels;

  const _ChannelPieCard({required this.channels});

  @override
  Widget build(BuildContext context) {
    final total = channels.fold<int>(0, (a, c) => a + c.sent);
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'By Channel',
            style: TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 24),
          if (total == 0)
            const Padding(
              padding: EdgeInsets.symmetric(vertical: 24),
              child: Text(
                'No data yet',
                style: TextStyle(color: AppColors.textSecondary),
              ),
            )
          else
            SizedBox(
              height: 200,
              child: PieChart(
                PieChartData(
                  sectionsSpace: 0,
                  centerSpaceRadius: 60,
                  sections: channels
                      .map(
                        (c) => PieChartSectionData(
                          color:
                              kChannelColors[c.channel] ?? AppColors.textMuted,
                          value: c.sent.toDouble(),
                          title: '',
                          radius: 30,
                        ),
                      )
                      .toList(),
                ),
              ),
            ),
          const SizedBox(height: 24),
          ...channels.map(
            (c) => _buildPieLegend(
              c.channel,
              total == 0 ? 0 : ((c.sent / total) * 100).round(),
              kChannelColors[c.channel] ?? AppColors.textMuted,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPieLegend(String label, int percent, Color color) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8.0),
      child: Row(
        children: [
          Container(
            width: 8,
            height: 8,
            decoration: BoxDecoration(color: color, shape: BoxShape.circle),
          ),
          const SizedBox(width: 8),
          Text(
            label,
            style: const TextStyle(
              color: AppColors.textSecondary,
              fontSize: 14,
            ),
          ),
          const Spacer(),
          Text(
            '$percent%',
            style: const TextStyle(
              color: AppColors.textPrimary,
              fontSize: 14,
              fontWeight: FontWeight.bold,
            ),
          ),
        ],
      ),
    );
  }
}

class _DlqTrendCard extends StatelessWidget {
  final List<DlqTrendPoint> dlqTrend;

  const _DlqTrendCard({required this.dlqTrend});

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
            height: 220,
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
                      titlesData: FlTitlesData(
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
                        leftTitles: const AxisTitles(
                          sideTitles: SideTitles(
                            showTitles: true,
                            reservedSize: 40,
                          ),
                        ),
                        topTitles: const AxisTitles(
                          sideTitles: SideTitles(showTitles: false),
                        ),
                        rightTitles: const AxisTitles(
                          sideTitles: SideTitles(showTitles: false),
                        ),
                      ),
                      borderData: FlBorderData(show: false),
                      gridData: FlGridData(
                        show: true,
                        getDrawingHorizontalLine: (value) => const FlLine(
                          color: AppColors.borderSubtle,
                          strokeWidth: 1,
                          dashArray: [5, 5],
                        ),
                      ),
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

class _ChannelDeliveryRatesCard extends StatelessWidget {
  final List<ChannelBreakdown> channels;

  const _ChannelDeliveryRatesCard({required this.channels});

  @override
  Widget build(BuildContext context) {
    return GlassCard(
      padding: const EdgeInsets.all(24),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'Delivery Rate by Channel',
            style: TextStyle(
              color: AppColors.textPrimary,
              fontSize: 18,
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          if (channels.isEmpty)
            const Text(
              'No data yet',
              style: TextStyle(color: AppColors.textSecondary),
            )
          else
            ...channels.map(
              (c) => Padding(
                padding: const EdgeInsets.only(bottom: 12),
                child: Row(
                  children: [
                    SizedBox(
                      width: 100,
                      child: Text(
                        c.channel,
                        style: const TextStyle(color: AppColors.textPrimary),
                      ),
                    ),
                    Expanded(
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(4),
                        child: LinearProgressIndicator(
                          value: (c.deliveryRate / 100).clamp(0, 1),
                          minHeight: 8,
                          backgroundColor: AppColors.surfaceObsidian,
                          color:
                              kChannelColors[c.channel] ?? AppColors.neonTeal,
                        ),
                      ),
                    ),
                    const SizedBox(width: 12),
                    Text(
                      '${c.deliveryRate.toStringAsFixed(1)}%',
                      style: const TextStyle(
                        color: AppColors.textPrimary,
                        fontWeight: FontWeight.bold,
                      ),
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
