<template>
  <div>
    <!-- 右下角固定按鈕 -->
    <div v-if="showButton" style="position: fixed; right: 20px; bottom: 80px; z-index: 2000;">
      <button class="btn btn-secondary rounded-pill shadow" @click="openModal">
        移除廣告24hr
      </button>
    </div>

    <!-- Bootstrap Modal -->
    <div class="modal fade" ref="removeAdModal" data-backdrop="static" data-keyboard="false" tabindex="-1" role="dialog" aria-labelledby="removeAdModalLabel" aria-hidden="true">
      <div class="modal-dialog" role="document">
        <div class="modal-content">
          <div class="modal-header">
            <h5 class="modal-title" id="removeAdModalLabel">移除廣告 24 小時</h5>
            <button type="button" class="close" @click="closeModal" aria-label="Close">
              <span aria-hidden="true">&times;</span>
            </button>
          </div>
          <div class="modal-body">
            <div id="remove-onead-ad-container">
              <iframe src="/onead-media" width="100%" height="250" frameborder="0" scrolling="no" style="border:0;display:block;"></iframe>
            </div>
            <div class="text-center mt-3">
              <span v-if="adLoading" class="text-secondary">廣告載入中...</span>
              <span v-if="adLoadFailed" class="text-danger">廣告載入失敗，請稍後再試</span>
              <span v-if="adLoaded && !adClicked" class="text-success">請點擊廣告完成一次互動</span>
              <span v-if="adClicked" class="text-primary">感謝您的支持</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  data() {
    return {
      adLoaded: false,
      adClicked: false,
      scriptLoaded: false,
      adLoading: false,
      adLoadFailed: false,
      adLoadTimeoutId: null,
      showButton: true,
    };
  },
  methods: {
    openModal() {
      $(this.$refs.removeAdModal).modal('show');
      this.adLoading = true;
      this.adLoadFailed = false;
      this.adLoaded = false;
      this.adClicked = false;
      if (this.adLoadTimeoutId) clearTimeout(this.adLoadTimeoutId);
      this.adLoadTimeoutId = setTimeout(() => {
        if (!this.adLoaded) {
          this.adLoadFailed = true;
          this.adLoading = false;
        }
      }, 5000);
      this.$nextTick(() => {
        // 監聽 iframe 載入與點擊
        const iframe = this.$el.querySelector('iframe');
        if (iframe) {
          iframe.onload = () => {
            this.adLoaded = true;
            this.adLoading = false;
            this.adLoadFailed = false;
            clearTimeout(this.adLoadTimeoutId);
            // 嘗試監聽 iframe 內容的點擊（僅同源可行）
            try {
              iframe.contentWindow.document.body.addEventListener('click', this.handleAdClick, { once: true });
            } catch (e) {
              // 若跨域，則無法直接監聽
              iframe.contentWindow.addEventListener('click', this.handleAdClick, { once: true });
            }
          };
        }
      });
    },
    closeModal() {
      $(this.$refs.removeAdModal).modal('hide');
      this.adLoaded = false;
      this.adClicked = false;
      this.adLoading = false;
      this.adLoadFailed = false;
      if (this.adLoadTimeoutId) clearTimeout(this.adLoadTimeoutId);
      this.showButton = false;
    },
    loadAdScript() {

    },
    handleAdClick() {
      console.log('Ad clicked');
      if (!this.adClicked) {
        this.adClicked = true;
        axios.post('/api/remove-ad-24hr');
      }
    }
  }
};
</script>
