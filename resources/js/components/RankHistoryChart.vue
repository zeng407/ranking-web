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
        'week': [],
        'all': [],
        'current': []
      },
    }
  },
  methods: {
    initChart() {
      Promise.all([
        this.loadRanks(['all','week']),
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
          this.ranks['week'] = response.data.week;
          this.ranks['current'] = response.data.current;
        }).catch(error => {
          console.log(error);
        });
    },
    drawChart(target) {
      // skip if target is not found
      if(!document.getElementById(target)){
        return;
      }
      const ctx = target;
      const weeklyRankData = this.ranks['week']
      .filter((item) => {
        if(item.rank === 0 || item.win_rate <= 0){
          return false;
        }

        return true;
      })
      .map((item, index) => {
        return {
          x: item.date,
          y: item.rank,
          win_rate: item.win_rate
        }
      });
      const allRankData = [this.ranks['current']]
        .concat(this.ranks['all'].filter((item) => item.date !== moment().format('YYYY-MM-DD')))
        .filter((item, index) => {
          if(item.rank === 0 || item.win_rate <= 0){
            return false;
          }

          if(item.date == moment().format('YYYY-MM-DD')){
            return true;
          }

          //  ture if index is 0 2 4
          if(index % 2 == 0 && index < 5){
            return true;
          }

          if(index % 4 == 0 && index >= 5){
            return true;
          }

          return weeklyRankData.some((item2) => {
            return item.date === item2.x;
          });

        })
        .map((item, index) => {
          return {
            x: item.date,
            y: item.rank,
            win_rate: item.win_rate
          }
        });


      new Chart(ctx, {
        type: 'line',
        data: {
          datasets: [{
            label: this.$t('rank.chart.title.rank_history.all'),
            data: allRankData,
            pointStyle: 'circle',
            borderColor: 'rgba(152, 204, 253, 0.5)',
            backgroundColor: 'rgba(152, 204, 253, 1)', //blue,
          }
          ,
          {
            label: this.$t('rank.chart.title.rank_history.week'),
            data: weeklyRankData,
            pointStyle: 'circle',
            borderColor: 'rgba(255, 99, 132, 0.5)',
            backgroundColor: 'rgba(255, 99, 132, 1)', //red,
          }
        ]
        },
        options: {
          responsive: true,
          scales: {
            x: {
              type: 'timeseries',
              time: {
                tooltipFormat: 'll',
                parser: 'YYYY-MM-DD',
                unit: 'day',
              },
              title: {
                display: true,
                text: 'Date'
              },
              ticks: {
                callback: (value, index, values) => {
                  moment.locale(this.$i18n.locale);
                  return moment(value).format('yyyy-MM-DD');
                },
                // if number is less 10, it will show all, if more than 10, it will show 10
                stepSize: () => {
                  if(this.ranks['week'].length < 10){
                    return 1;
                  }else{
                    return Math.ceil(this.ranks['week'].length / 10);
                  }
                },
              }
            },
            y: {
              suggestedMin: 1,
              type: 'linear',
              position: 'right',
              reverse: true,
              ticks:{
                stepSize: 1,
                precision: 0,
                callback: (value, index, values) => {
                  return '#'+value;
                },
              },
              afterDataLimits: (axis) => {
                axis.max = axis.max+5;
                axis.min = 1;
              }
            },
          },
          interaction: {
            intersect: false
          },
          plugins: {
            title: {
              display: false,
              text: this.$t('rank.chart.title.rank_history.week'),
            },
            tooltip: {
              callbacks: {
                label: (context) => {
                  const data = context.dataset.data[context.dataIndex];
                  let label = context.dataset.label + ' ' + this.$t('rank.chart.rank') + ': ' + data.y;
                  if(data.win_rate){
                    label += ' ' + this.$t('rank.chart.win_rate') + ': ' + data.win_rate + '%';
                  }
                  return label;
                }
              }
            }
          },
        }
      });
    },
    getChartWidth(){
      return 400 + this.ranks.week.length*8;
    },
    mergeRankData(){
      let allRankData = this.ranks['all'];
      let weeklyRankData = this.ranks['week'];

      // filter out the same date
      allRankData = allRankData.filter((item) => {
        return !weeklyRankData.some((item2) => {
          return item.date === item2.date;
        });
      });

      return {
        all: allRankData,
        week: weeklyRankData,
      };
    }
  }
}
</script>
