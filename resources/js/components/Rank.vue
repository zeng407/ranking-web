<script>
import Swal from 'sweetalert2';
import CountWords from './partials/CountWords.vue';
import Chart from 'chart.js/auto';
import 'chartjs-adapter-moment';

export default {
  components: {
    CountWords
  },
  mounted() {
    this.loadCommnets();
    this.initChart();
  },
  data() {
    return {
      commentInput: '',
      comments: [],
      meta: {
        current_page: 1,
        last_page: 1
      },
      profile: {
        nickname: '',
        avatar_url: '',
        champions: []
      },
      isSubmiting: false,
      viewerOptions: {
        inline: false,
        button: true,
        movable: true,
        navbar: 0,
        title: false,
        toolbar: {
          zoomIn: 1,
          zoomOut: 1,
          reset: 1,
          rotateRight: 1,
        },
        rotatable: true,
      },
      anonymous: false,
      chartLoaded: [],
      showMyTimeline: true,
      showRankHistory: true,
    }
  },
  props: {
    commentMaxLength: {
      type: String,
      required: true
    },
    indexCommentEndpoint: {
      type: String,
      required: true
    },
    createCommentEndpoint: {
      type: String,
      required: true
    },
    reportCommentEndpoint: {
      type: String,
      required: true
    },
    championHistories: {
      type: Object,
      required: true
    },
    maxRank: {
      type: Number,
      required: true
    },
    gameStatistic: {
      type: Object,
      required: true
    }
  },
  computed: {
    commentWords() {
      return this.commentInput.length;
    },
    validComment() {
      return this.commentInput.trim().length > 0 && this.commentInput.length <= this.commentMaxLength
    }
  },
  methods: {
    loadCommnets(page = 1) {
      const urlParams = {
        page: page
      }
      axios.get(this.indexCommentEndpoint, {
        params: urlParams
      })
        .then(response => {
          this.comments = response.data.data;
          this.meta = response.data.meta;
          this.profile = response.data.profile;
        })
        .catch(error => {
          // console.log(error);
          Swal.fire({
            title: 'Error!',
            text: this.$t('Something went wrong. Please try again later.'),
            icon: 'error',
          });
        });
    },
    clickTab(tab) {
      const urlParams = new URLSearchParams(window.location.search);
      urlParams.set('tab', tab);
      const newUrl = window.location.pathname + '?' + urlParams.toString();
      window.history.replaceState(null, null, newUrl);
    },
    submitComment() {
      this.isSubmiting = true;
      if (this.commentInput.length > 0 && this.commentInput.length <= this.commentMaxLength) {
        axios.post(this.createCommentEndpoint, {
          content: this.commentInput,
          anonymous: this.anonymous
        })
          .then(response => {
            this.commentInput = '';
            this.loadCommnets();
            // Scroll to the comment position
            this.scrollToComment();
          })
          .catch(error => {
            // console.log(error);
            Swal.fire({
              title: 'Error!',
              text: error.response.data.message,
              icon: 'error',
              button: 'OK'
            });
          }).finally(() => {
            this.isSubmiting = false;
          });
      }
    },
    changePage(page) {
      this.loadCommnets(page);
      this.scrollToComment();
    },
    scrollToComment() {
      const navbarHeight = 60;
      $("html, body").animate({ scrollTop: $('#comments-total').offset().top - navbarHeight }, 500);
    },
    reportComment(comment) {
      const commentId = comment.id;

      Swal.fire({
        title: this.$t('Are you sure?'),
        text: 'Are you sure you want to report this comment?',
        icon: 'warning',
        showCancelButton: true,
        confirmButtonText: 'Yes',
        cancelButtonText: 'No',
        html: `Comment: <br><b> ${comment.content} </b><br> <strong>By: <i>${comment.nickname}</i></strong>`,
        input: 'select',
        inputOptions: {
          'Spam': this.$t('Spam'),
          'Inappropriate': this.$t('Inappropriate'),
          'Hate Speech': this.$t('Hate Speech'),
          'Harassment': this.$t('Harassment'),
          'Other': this.$t('Other')
        },
        inputPlaceholder: this.$t("Please select a reason for reporting"),
      }).then((result) => {
        if (result.isConfirmed) {
          if (result.value === 'Other') {
            Swal.fire({
              title: this.$t('Please specify the reason'),
              input: 'text',
              inputPlaceholder: this.$t('Please specify the reason'),
              showCancelButton: true,
              confirmButtonText: this.$t('Submit'),
              cancelButtonText: this.$t('Cancel'),
              inputValidator: (value) => {
                if (!value) {
                  return this.$t('report_comment_other_reason_required');
                }
              }
            }).then((result) => {
              if (result.isConfirmed) {
                const reportReason = result.value;
                const payload = {
                  reason: reportReason
                }
                this.performReportingComment(commentId, payload);
              }
            });
            return;
          } else {
            const reportReason = result.value;
            const payload = {
              reason: this.$t(reportReason)
            }
            this.performReportingComment(commentId, payload);
          }
        }
      });
    },
    performReportingComment(commentId, payload) {
      axios.post(this.reportCommentEndpoint.replace('_comment_id', commentId), payload)
        .then(response => {
          Swal.fire({
            title: this.$t('Reported!'),
            icon: 'success'
          });
        })
        .catch(error => {
          // console.log(error);
          Swal.fire({
            title: 'Error!',
            text: this.$t('Something went wrong. Please try again later.'),
            icon: 'error',
          });
        });
    },
    share() {
      const url = window.location.origin + window.location.pathname + '?utm_medium=share_rank';
      if (navigator.share) {
        navigator.share({
          url: url
        }).catch(console.error);
      } else {
        this.$refs['popover'].$emit('open');
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 2000);
      }
    },
    shareResult() {

      //get parameter g
      const urlParams = new URLSearchParams(window.location.search);
      const g = urlParams.get('g');
      const url = window.location.origin + window.location.pathname + '?s=' + g + '&utm_medium=share_result';

      if (navigator.share) {
        navigator.share({
          url: url,
          title: this.$t('My Voting Game Result'),
          text: this.$t('Check out my result on this voting game!'),
        }).catch(console.error);
      } else {
        this.$refs['share-popover'].$emit('open');
        navigator.clipboard.writeText(url);
        setTimeout(() => {
          this.$root.$emit('bv::hide::popover');
        }, 2000);
      }
    },
    drawChart(target, container, data) {
      const ctx = target;
      const chartRankData = data.map((item, index) => {
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
            label: this.$t('rank.chart.rank'),
            data: chartRankData,
            yAxisID: 'y-axis-1',
            pointStyle: 'circle',
            borderColor: 'rgba(0, 0, 0, 0.5)', //black
            backgroundColor: '#FFFFFF', //white
          },
        ]
        },
        options: {
          responsive: true,
          animation: {
            onComplete: () => {
              this.autoScrollChartToEnd(container);
            }
          },
          layout: {
            padding: 10
          },
          scales: {
            x :{
              type: 'time',
              time: {
                tooltipFormat: 'll',
                minUnit: 'day',
                parser: 'YYYY-MM-DD',
                unit: 'day',
              },
              ticks: {
                callback: (value, index, values) => {
                  moment.locale(this.$i18n.locale);
                  return moment(value).format('yyyy-MM-DD');
                },
                // if number is less 10, it will show all, if more than 10, it will show 10 
                stepSize: () => {
                  if(chartRankData.length < 10){
                    return 1;
                  }else{
                    return Math.ceil(chartRankData.length / 10);
                  }
                },
              }
            },
            'y-axis-1': {
              type: 'linear',
              display: true,
              beginAtZero: true,
              position: 'right',
              reverse: true,
              min: 1,
              // max: this.maxRank,
              ticks:{
                stepSize: 1,
                precision: 0,
                callback: (value, index, values) => {
                  return '#'+value;
                },
                // autoSkip: false,
                // maxTicksLimit: 100,
              },

            },
            // 'y-axis-2': {
            //   max: 100,
            //   min: 1,
            //   type: 'linear',
            //   display: false,
            //   position: 'right',
            //   beginAtZero: true,
            //   reverse: false,
            //   grid: {
            //     drawOnChartArea: false, // only want the grid lines for one axis to show up
            //   },
            //   backgroundColor: 'rgba(255, 99, 132, 0.2)',
            //   ticks: {
            //     callback: function (value, index, values) {
            //       return Math.round(value) + '%';
            //     },
            //     stepSize: 1,
            //     precision: 0.1
            //   },
            // },
          },
          interaction: {
            intersect: false,
            mode: 'index',
          },
          plugins: {
            title: {
              display: true,
              text: this.getGlobalRankAxisDescription(),
            },
            legend: {
              display: false,
              position: 'chartArea',
              labels: {
                usePointStyle: true,
                pointStyle: 'circle',
                boxWidth: 10,
                boxHeight: 10,
                padding: 20,
              },
              onClick: ()=>{}
            },
            tooltip: {
              callbacks: {
                footer: (tooltipItems) => {
                  const winRate = tooltipItems[0].dataset.data[tooltipItems[0].dataIndex].win_rate;
                  return this.$t('rank.chart.win_rate') + ': ' + winRate + '%';
                }
              }
            },
          }
        }
      });
    },
    drawMyTimeline(target, container, data) {
      const ctx = target;
      const championHistories = data;
      const chartData = championHistories.map((item, index) => {
        return {
          x: item.rounds,
          y: item.diff,
          start_at: item.start_at,
          winner: item.winner_name,
          loser: item.loser_name,
          winner_id: item.winner,
        }
      });

      new Chart(ctx, {
        type: 'bar',
        data: {
          labels: chartData.map((item, index) => {
            return this.$t('rank.chart.round', {round: item.x});
          }),
          datasets: [{
            label: this.$t('rank.chart.thinking_time'),
            data: chartData,
            pointStyle: 'circle',
            backgroundColor: chartData.map((item, index) => {
              if(item.winner_id === this.gameStatistic.winner_id) {
                return 'rgba(54, 162, 235, 0.5)'; //blue
              } else {
                return 'rgba(0, 0, 0, 0.5)'; //black
              }
            }),
            borderColor: '#000000', //black
          }]
        },
        options: {
          responsive: true,
          animation: {
            onComplete: () => {
              this.autoScrollChartToEnd(container);
            }
          },
          layout: {
            padding: 10
          },
          scales: {
            x : {
              ticks: {
                callback: (value, index, values) => {
                  // For a category axis, the val is the index.
                  return this.$t('rank.chart.round', {round: value + 1});
                },
              }
            },
            y : {
              type: 'linear',
              display: true,
              beginAtZero: true,
              position: 'right',
              min: 0,
              max: Math.max(...chartData.map((item, index) => {
                if (item.y > 7200) {
                  return 7200;
                }
                return item.y;
              })),
              ticks:{
                stepSize: 1,
                precision: 0,
                callback: (value, index, values) => {
                  return this.secondsToHms(value);
                },
                // autoSkip: false,
                // maxTicksLimit: 100,
              },

            },
          },
          interaction: {
            intersect: false,
            mode: 'index',
          },
          plugins: {
            title: {
              display: true,
              text: this.$t('rank.chart.title.timeline'),
            },
            legend: {
              display: false,
              position: 'chartArea',
              labels: {
                usePointStyle: true,
                pointStyle: 'rect',
                boxWidth: 10,
                boxHeight: 10,
                padding: 20,
              },
              onClick: ()=>{}
            },
            tooltip: {
              callbacks: {
                label: (tooltipItems) => {
                  return this.$t('rank.chart.thinking_time') + ': ' + this.secondsToHms(tooltipItems.parsed.y);
                },
                footer: (tooltipItems) => {
                  // show the winner and loser
                  const winner = tooltipItems[0].dataset.data[tooltipItems[0].dataIndex].winner;
                  const loser = tooltipItems[0].dataset.data[tooltipItems[0].dataIndex].loser;
                  return this.$t('rank.chart.winner') + ': ' + winner + '\n' + this.$t('rank.chart.loser') + ': ' + loser;
                }
              }
            },
          }
        }
      });
    },
    initChart() {
      if (this.championHistories && this.championHistories['my']) {
        this.drawChart('my-champion', 'my-champion-container', this.championHistories['my']['data']);
      }
      if (this.championHistories && this.championHistories['global']) {
        this.drawChart('global-champion', 'global-champion-container', this.championHistories['global']['data']);
      }
      if (this.gameStatistic && this.gameStatistic['timeline']) {
        this.drawMyTimeline('my-timeline', 'my-timeline-container', this.gameStatistic['timeline']);
      }
    },
    getGlobalRankAxisDescription() {
      if(this.gameStatistic){
        return this.$t('rank.chart.title.rank_history');
      }else{
        return this.$t('rank.chart.title.rank_history');
      }
    },
    autoScrollChartToEnd(container) {
      if(!this.chartLoaded.includes(container)){
        this.chartLoaded.push(container);
        // scroll target to the end
        const targetElement = document.getElementById(container);
        if(!targetElement){
          return;
        }
        // scroll right smooth
        targetElement.scrollTo({
          left: targetElement.scrollWidth,
          behavior: 'smooth'
        });  
      }
    },
    secondsToHms(value) {
      // seconds to H:i:s
      const hours = Math.floor(value / 3600);
      const minutes = Math.floor((value % 3600) / 60);
      const seconds = value % 60;
      // don't show hours if it's 0
      if(hours > 0){
        return hours + 'h' + minutes + 'm' + seconds + 's';
      }else if(minutes > 0){
        return minutes + 'm' + seconds + 's';
      }else if(seconds > 0){
        return seconds + 's';
      }else{
        return '0s';
      }
    },
  }
}
</script>
