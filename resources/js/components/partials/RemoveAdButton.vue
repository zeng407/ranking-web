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
              <div id="div-onead-nd-00"></div>
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
        this.loadAdScript();
        window.custom_call = (params) => {
          if (params.hasAd) {
            this.adLoaded = true;
            this.adLoading = false;
            this.adLoadFailed = false;
            clearTimeout(this.adLoadTimeoutId);
            const parent = document.getElementById('remove-onead-ad-container');
            const adDiv = parent ? parent.firstElementChild : null;
            if (adDiv) {
              adDiv.addEventListener('click', this.handleAdClick, { once: true });
            }
          } else {
            this.adLoadFailed = true;
            this.adLoading = false;
            clearTimeout(this.adLoadTimeoutId);
          }
        };
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
      if (this.scriptLoaded) return;
      const script = document.createElement('script');
      script.type = 'text/javascript';
      script.innerHTML = `
        var custom_call = function(params) {
          if(params.hasAd && typeof window.custom_call === 'function') {
            window.custom_call(params);
          }
        };
        ONEAD_TEXT = {};
        ONEAD_TEXT.pub = {};
        ONEAD_TEXT.pub.uid = "2000374";
        ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-nd-00");
        ONEAD_TEXT.pub.player_mode = "native-drive";
        ONEAD_TEXT.pub.player_mode_div = "div-onead-ad";
        ONEAD_TEXT.pub.max_threads = 3;
        ONEAD_TEXT.pub.position_id = /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent)? "5" : "0";
        ONEAD_TEXT.pub.queryAdCallback = custom_call;
        window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
        ONEAD_text_pubs.push(ONEAD_TEXT);
      `;
      script.id = 'onead-config-script';
      document.body.appendChild(script);
      const adScript = document.createElement('script');
      adScript.src = 'https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js';
      adScript.id = 'onead-script';
      document.body.appendChild(adScript);
      this.scriptLoaded = true;
    },
    handleAdClick() {
      this.adClicked = true;
      axios.post('/api/remove-ad-24hr');
    }
  }
};
</script>
