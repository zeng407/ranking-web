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
      isGeneratingImage: false,
      cachedTop10: null
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
    downloadBlob(blob, filename) {
      // IE 10+
      if (navigator.msSaveBlob) {
        navigator.msSaveBlob(blob, filename);
        return;
      }

      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = filename;

      // For mobile devices, target="_blank" prevents the current page from being replaced
      // if the download doesn't trigger correctly or if the browser treats it as a navigation
      if (('ontouchstart' in window) || navigator.maxTouchPoints > 0) {
        a.target = '_blank';
      }

      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);

      // Release URL object after a longer delay to ensure download starts
      setTimeout(() => URL.revokeObjectURL(url), 20000);
    },
    async buildTop10Image(topCount = 10) {
      try {
        if (!this.gameResult) {
          Swal.fire({
            title: 'Error!',
            text: 'No game result found',
            icon: 'error',
          });
          return;
        }

        const maxCount = Math.max(1, Math.min(10, topCount));

        // Use cache if available
        if (this.cachedTop10 && this.cachedTop10.count === maxCount) {
          this.showGeneratedImageModal(this.cachedTop10.dataUrl, this.cachedTop10.rankingText, maxCount);
          return;
        }

        this.isGeneratingImage = true;

        // Assume we have top 10 data: ranks = [ {rank:1,img:url,title:""}, ... ]
        const ranks = [
          { rank: 1, img: this.gameResult.winner.thumb_url || this.gameResult.winner.imgur_url, title: this.gameResult.winner.title },
          ...this.gameResult.data.map((r,i) => ({
            rank: i+2, img: r.loser.thumb_url || r.loser.imgur_url, title: r.loser.title
          })).slice(0, maxCount - 1)
        ];

        const total = Math.min(ranks.length, maxCount);

        if (total === 0) {
          Swal.fire({
            title: 'Error!',
            text: 'No images to render',
            icon: 'error',
          });
          this.isGeneratingImage = false;
          return;
        }

        // Custom layout:
        // Row 1: Rank 1 (1200x800)
        // Row 2: Rank 2-4 (400x400 each)
        // Row 3: Rank 5-7 (400x300 each)
        // Row 4: Rank 8-10 (400x300 each)
        const margin = 20;
        const gap = 20;

        const slots = [];

        if (total >= 1) {
          slots[0] = { x: margin, y: margin, w: 1200 + margin * 2, h: 800 }; // Rank 1
        }

        // Row 2: Rank 2-4 (均匀分布, 400x400)
        const row2Y = margin + 800 + gap;
        for (let i = 1; i < Math.min(total, 4); i++) {
          slots[i] = {
            x: margin + (i - 1) * (400 + gap),
            y: row2Y,
            w: 400,
            h: 400
          };
        }

        // Row 3: Rank 5-7 (400x300 each)
        const row3Y = row2Y + 400 + gap;
        for (let i = 4; i < Math.min(total, 7); i++) {
          slots[i] = {
            x: margin + (i - 4) * (400 + gap),
            y: row3Y,
            w: 400,
            h: 300
          };
        }

        // Row 4: Rank 8-10 (400x300 each)
        const row4Y = row3Y + 300 + gap;
        for (let i = 7; i < Math.min(total, 10); i++) {
          slots[i] = {
            x: margin + (i - 7) * (400 + gap),
            y: row4Y,
            w: 400,
            h: 300
          };
        }

        // Calculate canvas size based on slots
        let canvasWidth = margin;
        let canvasHeight = margin;

        slots.forEach(slot => {
          canvasWidth = Math.max(canvasWidth, slot.x + slot.w + margin);
          canvasHeight = Math.max(canvasHeight, slot.y + slot.h + margin);
        });

        const canvasEl = document.createElement('canvas');
        const canvas = new Canvas(canvasEl, { width: canvasWidth, height: canvasHeight, backgroundColor: '#111' });

        const loadImage = (url, slot) => new Promise((resolve, reject) => {

          // Use proxy URL to avoid CORS issues if configured
          let imageUrl = this.useImageProxy ? `/proxy-image?url=${encodeURIComponent(url)}` : url;
          let retried = false;

          // Add timeout mechanism
          const timeout = setTimeout(() => {
            console.error('Image load timeout:', url);
            reject(new Error('Image load timeout'));
          }, 10000); // 10 second timeout

          // Load using native Image object
          const imgElement = new Image();
          imgElement.crossOrigin = 'anonymous'; // Allow cross-origin images in canvas

          imgElement.onload = () => {
            clearTimeout(timeout);

            // Create blurred background
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

            // Main image - scale proportionally and center
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
            // If proxy failed and we haven't tried direct URL yet, retry with direct URL
            if (this.useImageProxy && !retried && imageUrl.includes('/proxy-image')) {
              console.warn('Proxy failed, retrying with direct URL:', url);
              retried = true;
              imageUrl = url;
              imgElement.src = imageUrl;
            } else {
              console.error('Failed to load image:', url, error);
              reject(new Error('Failed to load image'));
            }
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

          // Top title text - no background box, use stroke
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

          // Top-left rank label
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

        // Generate ranking text list
        let rankingText = '';
        for (let i = 0; i < ranks.length && i < slots.length; i++) {
          const r = ranks[i];
          rankingText += `#${r.rank} ${r.title}\n`;
        }

        // Save to cache
        this.cachedTop10 = {
            count: maxCount,
            dataUrl: dataUrl,
            rankingText: rankingText
        };

        this.showGeneratedImageModal(dataUrl, rankingText, maxCount);
        this.isGeneratingImage = false;
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
    showGeneratedImageModal(dataUrl, rankingText, maxCount) {
        Swal.fire({
          imageUrl: dataUrl,
          imageAlt: 'Ranking Result',
          // dark background
          background: '#1a1a1a',
          // add download button logic
          showConfirmButton: false,
          customClass: {
             image: 'generated-rank-image'
          },
          didRender: (modal) => {
            const image = modal.querySelector('.generated-rank-image');
            if (image) {
              image.style.transition = 'all 0.4s cubic-bezier(0.175, 0.885, 0.32, 1.275)';
              image.style.borderRadius = '8px';
              image.style.border = '1px solid transparent';

              if (!('ontouchstart' in window)) {
                image.onmouseover = () => {
                  image.style.boxShadow = '0 15px 35px rgba(0, 212, 255, 0.15)';
                  image.style.border = '1px solid rgba(0, 212, 255, 0.3)';
                };
                image.onmouseout = () => {
                  image.style.boxShadow = 'none';
                  image.style.border = '1px solid transparent';
                };
              }
            }

            const downloadBtn = modal.querySelector('#downloadBtn');
            if (downloadBtn && !('ontouchstart' in window)) {
              downloadBtn.onmouseover = () => {
                downloadBtn.style.boxShadow = '0 0 30px rgba(0, 212, 255, 0.9), inset 0 0 20px rgba(255, 255, 255, 0.2)';
                downloadBtn.style.transform = 'translateY(-2px)';
              };
              downloadBtn.onmouseout = () => {
                downloadBtn.style.boxShadow = '0 0 20px rgba(0, 212, 255, 0.6), inset 0 0 20px rgba(255, 255, 255, 0.1)';
                downloadBtn.style.transform = 'translateY(0)';
              };
            }
          },
          showCloseButton: true,
          width: '480px',
          padding: '1em',
          html: `
            <div style="width: 100%; margin-bottom: 15px;">
              <button id="downloadBtn" style="
                background: linear-gradient(135deg, #00d4ff 0%, #00a9ff 50%, #0088ff 100%);
                color: #fff;
                border: 2px solid #00d4ff;
                padding: 12px 28px;
                font-weight: bold;
                border-radius: 8px;
                cursor: pointer;
                font-size: 16px;
                width: 100%;
                box-shadow: 0 0 20px rgba(0, 212, 255, 0.6), inset 0 0 20px rgba(255, 255, 255, 0.1);
                transition: all 0.3s ease;
                letter-spacing: 1px;">
                <i class="fa fa-download"></i>&nbsp; ${this.$t('Download Image')}
              </button>
            </div>
            <div style="text-align: left; color: #fff; max-height: 220px; overflow-y: auto;">
              <textarea id="rankingText" style="width: 100%; min-height: 100px; padding: 10px; background: #0a0e27; color: #00d4ff; border: 2px solid #00d4ff; border-radius: 6px; font-family: 'Courier New', monospace; resize: none; overflow: auto; box-shadow: inset 0 0 10px rgba(0, 212, 255, 0.2); font-size: 13px;" readonly>${rankingText}</textarea>
              <button id="copyBtn" style="margin-top: 10px; padding: 8px 16px; background: #333; color: #ddd; border: 1px solid #555; border-radius: 6px; cursor: pointer; width: 100%; font-size: 14px; transition: all 0.2s ease;">📋 ${this.$t('Copy result text')}</button>
            </div>
          `,
          didOpen: (modal) => {
            const downloadBtn = modal.querySelector('#downloadBtn');
            if (downloadBtn) {
              downloadBtn.addEventListener('click', (e) => {
                e.preventDefault();
                e.stopPropagation();

                // Re-create blob for download
                const byteString = atob(dataUrl.split(',')[1]);
                const mimeString = dataUrl.split(',')[0].split(':')[1].split(';')[0];
                const ab = new ArrayBuffer(byteString.length);
                const ia = new Uint8Array(ab);
                for (let i = 0; i < byteString.length; i++) {
                  ia[i] = byteString.charCodeAt(i);
                }
                const blob = new Blob([ab], { type: mimeString });
                this.downloadBlob(blob, `top${maxCount}.png`);
              });
            }

            const copyBtn = modal.querySelector('#copyBtn');
            if (copyBtn) {
              copyBtn.addEventListener('click', async (e) => {
                e.preventDefault();
                e.stopPropagation();

                let success = false;
                const textarea = modal.querySelector('#rankingText');

                // Helper for success UI
                const showSuccess = () => {
                  const originalText = copyBtn.textContent;
                  copyBtn.textContent = '✓ ' + this.$t('Copied');
                  copyBtn.style.background = '#28a745';
                  copyBtn.style.borderColor = '#28a745';
                  copyBtn.style.color = '#fff';

                  setTimeout(() => {
                    copyBtn.textContent = originalText;
                    copyBtn.style.background = '#333';
                    copyBtn.style.borderColor = '#555';
                    copyBtn.style.color = '#ddd';
                  }, 2000);
                };

                try {
                  // Try Clipboard API first
                  if (navigator.clipboard && navigator.clipboard.writeText) {
                    await navigator.clipboard.writeText(rankingText);
                    success = true;
                  } else {
                    throw new Error('Clipboard API unavailable');
                  }
                } catch (err) {
                  console.warn('Clipboard API failed, trying execCommand:', err);
                  // Fallback to execCommand
                  if (textarea) {
                    try {
                      textarea.select();
                      textarea.setSelectionRange(0, 99999); // For mobile devices
                      success = document.execCommand('copy');
                      window.getSelection().removeAllRanges();
                      textarea.blur();
                    } catch (execErr) {
                      console.error('execCommand failed:', execErr);
                    }
                  }
                }

                if (success) {
                  showSuccess();
                } else {
                  Swal.fire({
                    title: 'Error!',
                    text: 'Failed to copy ranking text. Please copy manually.',
                    icon: 'error',
                  });
                }
              });

              // Hover effects for copy button (desktop only)
              if (!('ontouchstart' in window)) {
                copyBtn.onmouseover = () => {
                  copyBtn.style.background = '#444';
                };
                copyBtn.onmouseout = () => {
                  copyBtn.style.background = '#333';
                };
              }
            }
          }
        }).then((result) => {
          // No confirm button logic needed anymore
        });
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
