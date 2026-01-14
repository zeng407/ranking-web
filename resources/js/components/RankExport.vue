<script>
import Swal from 'sweetalert2';
import { Canvas, Image as FabricImage, Rect, Text, filters } from 'fabric';

export default {
  props: {
    postSerial: {
      type: String,
      required: true
    },
    gameResult: {
      type: Object|null,
      default: null
    },
    requestHost: {
      type: String,
      required: true
    }
  },
  data() {
    return {
      isGeneratingImage: false,
      generatedImage: null,
      rankingText: '',
      topCount: 10,
      useImageProxy: false,
      copyButtonText: '📋 Copy'
    }
  },
  mounted() {
    this.copyButtonText = '📋 ' + this.$t('Copy');
    this.$nextTick(() => {
        this.buildTop10Image();
    });
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
    handleDownload() {
        if (!this.generatedImage) return;
        
        const dataUrl = this.generatedImage;
        const maxCount = this.topCount;

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
    },
    async copyRankingText() {
        if (!this.rankingText) return;

        let success = false;
        try {
            // Try Clipboard API first
            if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(this.rankingText);
            success = true;
            } else {
            throw new Error('Clipboard API unavailable');
            }
        } catch (err) {
            console.warn('Clipboard API failed, trying execCommand:', err);
            // Fallback to execCommand
            const textarea = document.getElementById('rankingTextarea');
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
            const originalText = this.copyButtonText;
            this.copyButtonText = '✓ ' + this.$t('Copied');
            setTimeout(() => {
                this.copyButtonText = originalText;
            }, 2000);
        } else {
            Swal.fire({
                title: 'Error!',
                text: 'Failed to copy ranking text. Please copy manually.',
                icon: 'error',
            });
        }
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
        this.topCount = maxCount;
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
        
        this.generatedImage = dataUrl;
        this.rankingText = rankingText;
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
  }
}
</script>
