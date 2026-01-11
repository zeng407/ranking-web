<script>
import Swal from 'sweetalert2';
import CountWords from './partials/CountWords.vue';
import Chart from 'chart.js/auto';
import ICountUp from 'vue-countup-v2';
import 'chartjs-adapter-moment';
import Vue from 'vue';
import { Canvas, Image as FabricImage, Rect, Text, filters } from 'fabric';

export default {
  components: {
    CountWords,
    ICountUp
  },
  mounted() {
    if(this.hasGameRoom){
      this.loadGameRoomRanks();
    }
    this.loadRankFromLocal();
    this.loadCommnets();
    this.initChart();
    this.enableTooltip();
    window.addEventListener('scroll', this.handleScroll);
  },
  beforeDestroy() {
    window.removeEventListener('scroll', this.handleScroll);
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
      showMyTimeline: false,
      showRankHistory: true,
      sortByTop: true,
      keyword: '',
      searchResults: [],
      gameRoomRanks: [],
      localRanks: [],
      // scrolling detection
      lastScrollPosition: 0,
      showReturnUpButton: false,
      // image generation config
      useImageProxy: false,
      isGeneratingImage: false
    }
  },
  props: {
    postSerial: {
      type: String,
      required: true
    },
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
    indexGameRoomRankEndpoint: {
      type: String|null,
      required: false,
      default: null
    },
    searchEndpoint: {
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
      type: Object|null
    },
    gameSerial: {
      type: String|null,
      default: null
    },
    requestHost: {
      type: String,
      required: true
    },
    hasGameRoom: {
      type: Boolean,
      required: true
    },
    gameResult: {
      type: Object|null,
      default: null
    },
  },
  computed: {
    commentWords() {
      return this.commentInput.length;
    },
    validComment() {
      return this.commentInput.trim().length > 0 && this.commentInput.length <= this.commentMaxLength
    },
    getSortedRanks() {
      if(!this.gameRoomRanks || !this.gameRoomRanks.ranks){
        return [];
      }
      if(this.sortByTop){
        return this.gameRoomRanks.ranks.top_10;
      }else{
        return this.gameRoomRanks.ranks.bottom_10;
      }
    },
    gameResultThumbs() {
      if (!this.gameResult) {
        return [];
      }
      const thumbs = [];
      const pushThumb = (el) => {
        if (!el) return;
        const url = el.thumb_url || el.imgur_url || el.mediumthumb_url || el.lowthumb_url;
        if (url) {
          thumbs.push(url);
        }
      };
      pushThumb(this.gameResult.winner);
      (this.gameResult.data || []).forEach((r) => pushThumb(r.loser));
      return thumbs.slice(0, 4);
    },
  },
  methods: {
    async buildTop10Image(topCount = 10) {
      try {
        this.isGeneratingImage = true;

        if (!this.gameResult) {
          Swal.fire({
            title: 'Error!',
            text: 'No game result found',
            icon: 'error',
          });
          this.isGeneratingImage = false;
          return;
        }

        // 假設已有前 10 名資料 ranks = [ {rank:1,img:url,title:""}, ... ]
        const maxCount = Math.max(1, Math.min(10, topCount));
        const ranks = [
          { rank: 1, img: this.gameResult.winner.thumb_url || this.gameResult.winner.imgur_url, title: this.gameResult.winner.title },
          ...this.gameResult.data.map((r,i) => ({
            rank: i+2, img: r.loser.thumb_url || r.loser.imgur_url, title: r.loser.title
          })).slice(0, maxCount - 1)
        ];

        // 兩欄布局，每排兩張圖
        const margin = 20;     // 上下左右邊距
        const gap = 20;        // 圖片間距
        const slotW = 600;     // 單張寬度
        const slotH = 400;     // 單張高度
        const cols = 2;
        const total = Math.min(ranks.length, maxCount);
        const rows = Math.ceil(total / cols);

        if (total === 0) {
          Swal.fire({
            title: 'Error!',
            text: 'No images to render',
            icon: 'error',
          });
          this.isGeneratingImage = false;
          return;
        }

        // 計算畫布尺寸：固定兩欄，行數依內容決定
        const canvasWidth = margin * 2 + cols * slotW + (cols - 1) * gap;
        const canvasHeight = margin * 2 + rows * slotH + (rows - 1) * gap;

        // 產生槽位位置（左右兩欄）
        const slots = Array.from({ length: total }, (_, i) => {
          const row = Math.floor(i / cols);
          const col = i % cols;
          return {
            w: slotW,
            h: slotH,
            x: margin + col * (slotW + gap),
            y: margin + row * (slotH + gap),
          };
        });

        const canvasEl = document.createElement('canvas');
        const canvas = new Canvas(canvasEl, { width: canvasWidth, height: canvasHeight, backgroundColor: '#111' });

        const loadImage = (url, slot) => new Promise((resolve, reject) => {

          // 根據設定決定是否使用代理 URL 避免 CORS 問題
          const imageUrl = this.useImageProxy ? `/proxy-image?url=${encodeURIComponent(url)}` : url;

          // 添加超时机制
          const timeout = setTimeout(() => {
            console.error('Image load timeout:', url);
            reject(new Error('Image load timeout'));
          }, 10000); // 10秒超时

          // 使用原生 Image 对象加载
          const imgElement = new Image();
          imgElement.crossOrigin = 'anonymous'; // 允许跨域图像在 canvas 中使用

          imgElement.onload = () => {
            clearTimeout(timeout);

            // 創建模糊背景
            const bgImg = new FabricImage(imgElement);
            const bgScaleX = slot.w / bgImg.width;
            const bgScaleY = slot.h / bgImg.height;
            bgImg.set({
              left: slot.x,
              top: slot.y,
              scaleX: bgScaleX,
              scaleY: bgScaleY,
              originX: 'left',
              originY: 'top',
              selectable: false,
            });
            bgImg.filters = [new filters.Blur({ blur: 0.8 })];
            bgImg.applyFilters();

            // 主圖片 - 等比例縮放並居中
            const mainImg = new FabricImage(imgElement);
            const scale = Math.min(slot.w / mainImg.width, slot.h / mainImg.height);
            const scaledW = mainImg.width * scale;
            const scaledH = mainImg.height * scale;

            mainImg.set({
              left: slot.x + (slot.w - scaledW) / 2,
              top: slot.y + (slot.h - scaledH) / 2,
              scaleX: scale,
              scaleY: scale,
              originX: 'left',
              originY: 'top',
              selectable: false
            });

            resolve({ bg: bgImg, main: mainImg });
          };

          imgElement.onerror = (error) => {
            clearTimeout(timeout);
            console.error('Failed to load image:', url, error);
            reject(new Error('Failed to load image'));
          };

          imgElement.src = imageUrl;
        });

        for (let i = 0; i < ranks.length && i < slots.length; i++) {
          const slot = slots[i];
          const r = ranks[i];
          if (!r.img) {
            console.warn('No image for rank', i+1);
            continue;
          }

          try {
            const { bg, main } = await loadImage(r.img, slot);
            canvas.add(bg);
            canvas.add(main);
          } catch (error) {
            console.error(`Failed to load image for rank ${i+1}:`, error);
            continue;
          }

          // 上方標題文字 - 移除背景框，使用描邊
          let titleText = r.title || '';
          if (titleText.length > 25) {
            titleText = titleText.substring(0, 25) + '...';
          }

          const title = new Text(titleText, {
            left: slot.x + 30,
            top: slot.y + 5,
            fill: '#fff',
            fontSize: 18,
            fontFamily: 'Arial',
            fontWeight: 'bold',
            stroke: '#000',
            strokeWidth: 4,
            paintFirst: 'stroke',
            originX: 'left',
            originY: 'top',
            selectable: false
          });
          canvas.add(title);

          // 左上角排名標籤
          const rankText = new Text('#' + r.rank, {
            left: slot.x + 5,
            top: slot.y + 5,
            fill: '#fff',
            fontSize: 32,
            fontFamily: 'Arial',
            fontWeight: 'bold',
            stroke: '#000',
            strokeWidth: 3,
            paintFirst: 'stroke',
            selectable: false
          });
          canvas.add(rankText);
        }

        console.log('Generating data URL...');
        const dataUrl = canvas.toDataURL({ format: 'png', quality: 0.92 });
        const a = document.createElement('a');
        a.href = dataUrl;
        a.download = `top${maxCount}.png`;
        a.click();

        this.isGeneratingImage = false;
        console.log('Image generation complete');
      } catch (error) {
        console.error('Error in buildTop10Image:', error);
        this.isGeneratingImage = false;
        Swal.fire({
          title: 'Error!',
          text: 'Failed to generate image: ' + error.message,
          icon: 'error',
        });
      }
    },
    loadRankFromLocal() {
      const key = `gamestate_${this.postSerial}`;
      const savedData = localStorage.getItem(key);

      if (savedData) {
        try {
            const parsedData = JSON.parse(savedData);
            const elements = parsedData.localElements;
            elements.sort((a, b) => b.local_win_count - a.local_win_count);
            const top10 = elements.slice(0, 10);
            this.localRanks = top10;
            return top10;
        } catch (e) {
            console.error('Failed to parse saved game state from localStorage:', e);
            return false;
        }
      }
    },
    search(){
      const inputValue = this.keyword.trim();
      const params = {
        keyword: inputValue,
        post_serial: this.postSerial
      };

      axios.get(this.searchEndpoint, {
        params: params
      })
        .then(response => {
          this.searchResults = response.data.data;

          // Show the modal
          $('#searchModal').modal('show');
        })
    },
    loadGameRoomRanks() {
      if (!this.indexGameRoomRankEndpoint || !this.gameSerial) {
        return;
      }
      const url = this.indexGameRoomRankEndpoint.replace('_game_serial', this.gameSerial);
      axios.get(url)
        .then(response => {
          this.gameRoomRanks = response.data;
          if(this.gameRoomRanks.rank_updating) {
            setTimeout(() => {
              this.loadGameRoomRanks();
            }, 5000);
          }
        });
    },
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
      const url = window.location.origin + window.location.pathname;
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
    shareRankLink() {
      return window.location.origin + window.location.pathname;
    },
    shareResultLink() {
      const urlParams = new URLSearchParams(window.location.search);
      const g = urlParams.get('g');
      const url = window.location.origin + window.location.pathname + '?s=' + g;
      return url;
    },
    shareGameLink() {
      return window.location.origin + '/g/' + this.postSerial;
    },
    handleScroll(event) {
      this.recordLastScrollPosition();
    },
    recordLastScrollPosition() {
      const currentScrollPosition = window.scrollY;
      if(this.recording == null){
        this.recording = setTimeout(() => {
          this.lastScrollPosition = currentScrollPosition;
          this.recording = null;
        }, 100);
      }

      if(this.isScrollingUp()) {
        this.showReturnUpButton = true;
      }else{
        this.showReturnUpButton = false;
      }
    },
    scrollToTop() {
      window.scrollTo({
        top: 0,
        behavior: 'smooth'
      });
    },
    isScrollingUp() {
      return window.scrollY < this.lastScrollPosition && window.scrollY > 300;
    },
    drawMyTimeline(target, container, data) {
      // skip if target is not found
      if(!document.getElementById(target)){
        return;
      }
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
          current_round: item.current_round,
          of_round: item.of_round
        }
      });

      new Chart(ctx, {
        type: 'bar',
        data: {
          labels: chartData.map((item, index) => {
            let text = '';
            let roundKeys = {
              1: 'game_round_final',
              2: 'game_round_semifinal',
              4: 'game_round_quarterfinal',
              8: 'game_round_of',
              16: 'game_round_of',
              32: 'game_round_of',
              64: 'game_round_of',
              128: 'game_round_of',
              256: 'game_round_of',
              512: 'game_round_of',
              1024: 'game_round_of'
            };
            let round = Object.keys(roundKeys).find(r => item.of_round <= r);
            if (round) {
              text = this.$t(roundKeys[round], { round: round });
            } else {
              text = this.$t('game_round_of', { round: item.of_round });
            }

            return this.$t('rank.chart.round', {round: item.x}) + `  (${text} ${item.current_round}/${item.of_round})`;
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
          maintainAspectRatio: false,
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

              },
            },
            y : {
              type: 'logarithmic',
              display: true,
              beginAtZero: true,
              position: 'right',
              min: 0,
              max: Math.max(...chartData.map((item, index) => {
                if (item.y > 3600) {
                  return 3600;
                }
                return item.y;
              })),
              suggestedMin: 1,
              ticks:{
                stepSize: 1,
                precision: 1,
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
              display: false,
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
              enabled: false,
              tooltip: {
                mode: 'index',
                intersect: false,
              },
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
              },
              external: (context) => {
                const getOrCreateTooltip = (chart) => {

                  // let tooltipEl = chart.canvas.parentNode.querySelector('div');
                  let tooltipEl = document.getElementById('chartjs-tooltip');
                  tooltipEl.style.display = 'block';
                  return tooltipEl;
                };

                const externalTooltipHandler = (context) => {
                  // Tooltip Element
                  const {chart, tooltip} = context;
                  const tooltipEl = getOrCreateTooltip(chart);



                  // Set Text
                  if (tooltip.body) {
                    const titleLines = tooltip.title || [];
                    const bodyLines = tooltip.body.map(b => b.lines);

                    const tableHead = document.createElement('thead');

                    titleLines.forEach(title => {
                      const tr = document.createElement('tr');
                      tr.style.borderWidth = 0;

                      const th = document.createElement('th');
                      th.style.borderWidth = 0;
                      const text = document.createTextNode(title);

                      th.appendChild(text);
                      tr.appendChild(th);
                      tableHead.appendChild(tr);
                    });

                    const tableBody = document.createElement('tbody');
                    bodyLines.forEach((body, i) => {
                      const colors = tooltip.labelColors[i];

                      const span = document.createElement('span');
                      span.style.background = colors.backgroundColor;
                      span.style.borderColor = '#fff';
                      span.style.borderStyle = 'solid';
                      span.style.borderWidth = '1px';
                      span.style.marginRight = '10px';
                      span.style.height = '10px';
                      span.style.width = '10px';
                      span.style.display = 'inline-block';

                      const tr = document.createElement('tr');
                      tr.style.backgroundColor = 'inherit';
                      tr.style.borderWidth = 0;

                      const td = document.createElement('td');
                      td.style.borderWidth = 0;

                      const text = document.createTextNode(body);

                      td.appendChild(span);
                      td.appendChild(text);
                      tr.appendChild(td);
                      tableBody.appendChild(tr);
                    });

                    // merge footer
                    const footerLines = tooltip.footer || [];
                    const tableFoot = document.createElement('tfoot');

                    footerLines.forEach(line => {
                      const tr = document.createElement('tr');
                      tr.style.borderWidth = 0;

                      const th = document.createElement('th');
                      th.style.borderWidth = 0;

                      if(line.startsWith(this.$t('rank.chart.winner'))){
                        // replace the first apperance
                        line = line.replace(this.$t('rank.chart.winner')+':', '');
                        // put a icon instead of text
                        const icon = document.createElement('i');
                        icon.classList.add('fa-solid', 'fa-thumbs-up');
                        icon.style.marginRight = '5px';
                        icon.style.width = '10px';
                        th.appendChild(icon);
                      }

                      if(line.includes(this.$t('rank.chart.loser'))){
                        line = line.replace(this.$t('rank.chart.loser')+':', '');
                        // put a icon instead of text
                        const icon = document.createElement('i');
                        icon.classList.add('fa-solid', 'fa-xmark');
                        icon.style.marginRight = '5px';
                        icon.style.width = '10px';
                        th.appendChild(icon);
                      }

                      let text = document.createTextNode(line);

                      th.appendChild(text);
                      tr.appendChild(th);
                      tableFoot.appendChild(tr);
                    });

                    const tableRoot = tooltipEl.querySelector('table');

                    // Remove old children
                    while (tableRoot.firstChild) {
                      tableRoot.firstChild.remove();
                    }

                    // Add new children
                    tableRoot.appendChild(tableHead);
                    tableRoot.appendChild(tableBody);
                    tableRoot.appendChild(tableFoot);
                  }

                  const {offsetLeft: positionX, offsetTop: positionY} = chart.canvas;

                  // Display, position, and set styles for font
                  tooltipEl.style.opacity = 1;
                  // tooltipEl.style.left = positionX + tooltip.caretX + 'px';
                  // tooltipEl.style.top = positionY + tooltip.caretY + 'px';
                  tooltipEl.style.font = tooltip.options.bodyFont.string;
                  // tooltipEl.style.padding = tooltip.options.padding + 'px ' + tooltip.options.padding + 'px';
                };

                externalTooltipHandler(context);
              }
            },
          }
        }
      });
    },
    initChart() {
      if (this.gameStatistic && this.gameStatistic['timeline']) {
        this.drawMyTimeline('my-timeline', 'my-timeline-container', this.gameStatistic['timeline']);
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
    computeAverageTime() {
      if(this.gameStatistic){
        const times = this.gameStatistic['timeline'].map((item, index) => {
          return item.diff;
        });
        const average = Math.round(times.reduce((a, b) => a + b, 0) / times.length);
        return this.secondsToHms(average);
      }
      return '0s';
    },
    computeMedianTime() {
      if(this.gameStatistic){
        const times = this.gameStatistic['timeline'].map((item, index) => {
          return item.diff;
        });
        const median = this.median(times);
        return this.secondsToHms(median);
      }
      return '0s';
    },
    median(values) {
      values.sort((a, b) => a - b);
      const half = Math.floor(values.length / 2);
      if (values.length % 2) {
        return values[half];
      }
      return (values[half - 1] + values[half]) / 2.0;
    },
    enableTooltip(){
      Vue.nextTick(() => {
        $(function () {
          $('[data-toggle="tooltip"]').tooltip()
        })
      });
    },
    changeSortRanks() {
      this.sortByTop = !this.sortByTop;
    },
    inject_youtube_embed(embedCode, params = {}){
      const width = params.width || '100%';
      const height = params.height || '270';

      // Replace width and height
      embedCode = embedCode.replace(/width="[\d%]+"/, `width="${width}"`);
      embedCode = embedCode.replace(/height="[\d%]+"/, `height="${height}"`);

      if (params.autoplay === false) {
        embedCode = embedCode.replace('autoplay=1', 'autoplay=0');
      }

      return embedCode;
    }
  }
}
</script>
