<template>
  <div class="hide-scrollbar-md overflow-x-scroll">
      <div class="d-flex align-content-center align-items-center justify-content-center p-0 rank-chart-container" :style="'min-width:'+ getChartWidth()">
        <canvas :id="chartId"></canvas>
      </div>
  </div>
</template>

<script>
import Chart from 'chart.js/auto';
import 'chartjs-adapter-moment';
import moment from 'moment';

export default {
  mounted() {
    this.initChart();
  },
  props: {
    chartId: {
      type: String,
      required: true
    },
    postSerial: {
      type: String,
      required: true
    },
    elementId: {
      type: String,
      required: true
    },
    indexRankEndpoint: {
      type: String,
      required: true
    },
  },
  data: function () {
    return {
      origin: window.location.origin,
      host: window.location.host,
      showMyTimeline: true,
      ranks: {
        'thousand_votes': [],
        'all': [],
        'current': []
      },
    }
  },
  methods: {
    initChart() {
      Promise.all([
        this.loadRanks(['all','thousand_votes']),
      ]).then(() => {
        this.drawChart(this.chartId);
      });

    },
    loadRanks(time) {
      const params = {
        time: time,
        post_serial: this.postSerial,
        element_id: this.elementId,
      };
      return axios.get(this.indexRankEndpoint,{
          params: params
        }).then(response => {
          this.ranks['all'] = response.data.all;
          this.ranks['thousand_votes'] = response.data.thousand_votes;
          this.ranks['current'] = response.data.current;
        }).catch(error => {
          console.log(error);
        });
    },
    drawChart(target) {
      // find canvas
      const canvas = document.getElementById(target);
      if (!canvas) return;
      const ctx = canvas.getContext('2d');

      const today = moment().format('YYYY-MM-DD');

      const thousandVotesRankData = (this.ranks['thousand_votes'] || [])
        .filter((item) => {
          if (!item) return false;
          if (item.rank === 0 || item.win_rate <= 0) return false;
          if (moment(item.date).isAfter(moment())) return false;
          return true;
        })
        .map((item) => ({ x: item.date, y: item.rank, win_rate: item.win_rate }));

      let allRankData = [];
      if (this.ranks['current']) {
        allRankData.push(this.ranks['current']);
      }
      if (Array.isArray(this.ranks['all'])) {
        allRankData = allRankData.concat(this.ranks['all'].filter((item) => {
          if (item.rank === 0) return false;
          if (moment(item.date).isAfter(moment())) return false;
          return item.date !== today
        }));
      }
      allRankData = allRankData.map((item) => ({ x: item.date, y: item.rank, win_rate: item.win_rate }));

      new Chart(ctx, {
        type: 'line',
        data: {
          datasets: [
            {
              label: this.$t('rank.chart.title.rank_history.all'),
              data: allRankData,
              pointStyle: 'circle',
              borderColor: 'rgba(152, 204, 253, 0.8)',
              backgroundColor: (context) => {
                const chart = context.chart;
                const { ctx, chartArea } = chart;
                if (!chartArea) return 'rgba(152, 204, 253, 0.15)';
                const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom);
                gradient.addColorStop(0, 'rgba(152, 204, 253, 0.25)');
                gradient.addColorStop(1, 'rgba(152, 204, 253, 0.0)');
                return gradient;
              },
              tension: 0.3,
              fill: true,
              borderWidth: 2,
              pointRadius: (ctx) => (ctx.dataIndex === 0 ? 4 : 2),
              pointHoverRadius: 5,
              cubicInterpolationMode: 'monotone',
              borderDash: [4, 4],
            },
            {
              label: this.$t('rank.chart.title.rank_history.thousand_votes'),
              data: thousandVotesRankData,
              pointStyle: 'circle',
              borderColor: 'rgba(255, 99, 132, 0.9)',
              backgroundColor: (context) => {
                const chart = context.chart;
                const { ctx, chartArea } = chart;
                if (!chartArea) return 'rgba(255, 99, 132, 0.15)';
                const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom);
                gradient.addColorStop(0, 'rgba(255, 99, 132, 0.25)');
                gradient.addColorStop(1, 'rgba(255, 99, 132, 0.0)');
                return gradient;
              },
              tension: 0.35,
              fill: true,
              borderWidth: 2,
              pointRadius: (ctx) => (ctx.dataIndex === (thousandVotesRankData.length - 1) ? 4 : 2),
              pointHoverRadius: 5,
              cubicInterpolationMode: 'monotone',
            },
          ],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          scales: {
            x: {
              type: 'timeseries',
              time: {
                tooltipFormat: 'YYYY-MM-DD',
                parser: 'YYYY-MM-DD',
                unit: 'day',
                displayFormats: {
                  day: 'YYYY-MM-DD'
                }
              },
              grid: {
                color: 'rgba(107, 114, 128, 0.1)',
              },
              ticks: {
                callback: (value) => {
                  moment.locale(this.$i18n.locale);
                  return moment(value).format('YYYY-MM-DD');
                },
              },
            },
            y: {
              suggestedMin: 1,
              type: 'linear',
              position: 'right',
              reverse: true,
              ticks: {
                stepSize: 1,
                precision: 0,
                callback: (value) => '#' + value,
              },
              afterDataLimits: (axis) => {
                axis.max = axis.max + 5;
                axis.min = 1;
              },
            },
          },
          interaction: {
            intersect: false,
            mode: 'index',
          },
          plugins: {
            legend: {
              position: 'top',
              labels: {
                usePointStyle: true,
                boxWidth: 8,
                padding: 16,
              },
            },
            decimation: {
              enabled: true,
              algorithm: 'lttb',
              samples: 60,
            },
            title: {
              display: false,
              text: this.$t('rank.chart.title.rank_history.thousand_votes'),
            },
            tooltip: {
              callbacks: {
                label: (context) => {
                  const data = context.dataset.data[context.dataIndex];
                  let label = context.dataset.label + ' ' + this.$t('rank.chart.rank') + ': ' + data.y;
                  if (data.win_rate) {
                    label += ' ' + this.$t('rank.chart.win_rate') + ': ' + data.win_rate + '%';
                  }
                  return label;
                },
              },
            },
          },
        },
      });
    },
    getChartWidth(){
      return 400 + ((this.ranks['thousand_votes'] || []).length * 8);
    },
    mergeRankData(){
      let allRankData = this.ranks['all'];
      let thousandVotesRankData = this.ranks['thousand_votes'];

      return {
        all: allRankData,
        thousand_votes: thousandVotesRankData,
      };
    }
  }
}
</script>
