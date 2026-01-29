<template>
  <img ref="flexImage" class="user-select-none" :src="currentDisplaySrc" @error="onImageError" :loading="loadingType">
</template>

<script>
import axios from 'axios'; // 確保有引入 axios

export default {
  mounted() {
    this.initImgAttributes();
    this.startImageLoadingProcess();
  },
  props: {
    elementId: { type: String | Number, required: true },
    thumbUrl: { type: String, required: true },
    thumbUrl2: { type: String },
    imgurUrl: { type: String },
    alt: { type: String },
    height: { type: String | Number },
    imageKey: { type: String | Number },
    handleLoaded: { type: Function, default: () => { } },
    srcSet: { type: String },
    sizes: { type: String },
    customClass: { type: String },
    lazy: { type: Boolean, default: false },
  },
  data() {
    return {
      reportUrl: "/api/image/report/removed",
      // 狀態標記
      isImgurUrlFailed: false,
      isThumbUrlFailed: false,
      isThumbUrl2Failed: false,
      isReported: false,
      // 新增：高解析度圖片是否已準備好
      isHighResReady: false,
    }
  },
  computed: {
    // 1. 計算最佳的高解析度 URL (含 Proxy 處理)
    computedHighResUrl() {
      if (!this.imgurUrl) return null;
      if (this.imgurUrl.startsWith('https://i.imgur.com/')) {
        return 'https://proxy.duckduckgo.com/iu/?u=' + encodeURIComponent(this.imgurUrl);
      }
      return this.imgurUrl;
    },
    // 2. 計算目前可用的低解析度 URL
    computedLowResUrl() {
      if (this.thumbUrl && !this.isThumbUrlFailed) {
        return this.thumbUrl;
      } else if (this.thumbUrl2 && !this.isThumbUrl2Failed) {
        return this.thumbUrl2;
      }
      return ''; // 真的都沒有時
    },
    // 3. 決定最終顯示在 img 標籤上的 src
    currentDisplaySrc() {
      // 邏輯：如果高解析沒掛掉 且 已經準備好(預載完成) -> 顯示高解析
      if (this.computedHighResUrl && !this.isImgurUrlFailed && this.isHighResReady) {
        return this.computedHighResUrl;
      }
      // 否則 -> 顯示低解析
      return this.computedLowResUrl;
    },
    loadingType() {
      return this.lazy ? 'lazy' : 'eager';
    }
  },
  watch: {
    // 如果 elementId 變了(例如在列表循環重用組件)，要重置狀態
    elementId() {
      this.isHighResReady = false;
      this.isImgurUrlFailed = false;
      this.isThumbUrlFailed = false;
      this.isThumbUrl2Failed = false;
      this.startImageLoadingProcess();
    }
  },
  methods: {
    initImgAttributes() {
      const img = this.$refs.flexImage;
      if (!img) return;

      if (this.height === 'auto') img.style.height = 'auto';
      else if (this.height > 0) img.style.height = this.height + 'px';

      if (this.customClass) img.classList.add(this.customClass);
      if (this.sizes) img.setAttribute('sizes', this.sizes);
      if (this.srcSet) img.setAttribute('srcset', this.srcSet);
      if (this.imageKey) img.setAttribute('key', this.imageKey);
      if (this.alt) img.setAttribute('alt', this.alt);

      // 注意：handleLoaded 會在目前的 src 載入完成時觸發
      // 因為我們會切換 src (低 -> 高)，所以可能會觸發兩次，這是正常的漸進式載入行為
      if (this.handleLoaded) {
        img.addEventListener('load', this.handleLoaded);
      }
    },

    // 啟動圖片載入流程
    startImageLoadingProcess() {
      // 1. 畫面預設會透過 computedLowResUrl 顯示低解析圖

      // 2. 如果有高解析圖，開始背景檢查與下載
      if (this.imgurUrl) {
        this.checkAndPreloadImgur();
      }
    },

    checkAndPreloadImgur() {
      // 步驟 A: 先檢查連結是否有效 (原本的邏輯)
      fetch(this.imgurUrl)
        .then(response => {
          if (!response.ok) {
            throw new Error('Network response was not ok');
          }
          // 檢查是否被導向到 removed.png
          if (response.url === 'https://i.imgur.com/removed.png') {
            this.isImgurUrlFailed = true;
            if (!this.isReported) {
              this.reportRemovedImage(this.imgurUrl);
              this.isReported = true;
            }
            return; // 終止，不進行預載
          }

          // 步驟 B: 連結有效，開始 "背景預載" 真實要顯示的圖片 (包含 Proxy 的那串網址)
          this.preloadHighResImage(this.computedHighResUrl);
        })
        .catch(error => {
          this.isImgurUrlFailed = true;
        });
    },

    preloadHighResImage(url) {
      const img = new Image();
      img.src = url;

      img.onload = () => {
        // 圖片完全下載完畢，切換顯示狀態
        this.isHighResReady = true;
      };

      img.onerror = () => {
        // 預載失敗，標記錯誤，UI 會維持在低解析度
        this.isImgurUrlFailed = true;
      };
    },

    onImageError() {
      // 這裡是處理 <img src="..."> 實際顯示失敗的情況
      const currentSrc = this.currentDisplaySrc;

      if (currentSrc === this.computedHighResUrl) {
        this.isImgurUrlFailed = true;
        this.isHighResReady = false; // 回退到低解析
      } else if (currentSrc === this.thumbUrl) {
        this.isThumbUrlFailed = true;
      } else if (currentSrc === this.thumbUrl2) {
        this.isThumbUrl2Failed = true;
      }
    },

    reportRemovedImage() {
      axios.post(this.reportUrl, {
        element_id: this.elementId
      }).catch(err => console.error(err));
    },
  }
};
</script>
