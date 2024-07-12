<template>
  <div ref="root">
    <ins class="adsbygoogle" ref="ad"
      style="display:block"
      :style="insStyle"
      :data-ad-client="pusherId"
      :data-ad-slot="slotId"
      :data-ad-format="adFormat"
      :data-full-width-responsive="fullWidth">
    </ins>
  </div>
</template>

<script>
export default {

  mounted() {
    this.loadScript();
  },
  props: {
    pusherId: {
      type: String,
      required: true
    },
    slotId: {
      type: String,
      required: true
    },
    insStyle: {
      type: String,
      required: false
    },
    fullWidth:{
      type: Boolean,
      required: false
    },
    adFormat: {
      type: String,
      required: false,
      default: 'auto'
    }

  },
  data: function () {
    return {

    }
  },
  methods: {
    loadScript() {
      const script = document.createElement('script');
      script.src = 'https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client='+this.pusherId;
      script.async = true;
      script.onload = () => this.loadAd(5,200);
      this.$refs.root.appendChild(script);
    },
    loadAd(retryCount, intervalDelay) {
      let retry = retryCount;
      let interval = setInterval(() => {
        try {
          // console.log('try to push ad, try: ' + retry);
          retry--;
          (adsbygoogle = window.adsbygoogle || []).push({});

          //if ad is loaded, then it will throw an error
          (adsbygoogle = window.adsbygoogle || []).push({});
        } catch (e) {
          if (e.message.includes(
            `All 'ins' elements in the DOM with class=adsbygoogle already have ads in them`)) {
            // after loaded ad, send a event to Home.vue
            window.dispatchEvent(new Event('ad-loaded'));
            clearInterval(interval);
            return;
          }
        }
        if (retry <= 0) {
          clearInterval(interval);
        }
      }, intervalDelay);

      retry = 4;
      let relocation = setInterval(() => {
        try {
          retry--;
          let ad = this.$refs.ad;
          if (ad) {
            ad.addClass('d-flex justify-content-center');
            clearInterval(relocation);
          }
          if (retry <= 0) {
            clearInterval(relocation);
          }
        } catch (e) {
          if (retry <= 0) {
            clearInterval(relocation);
          }
        }
      }, 500);
    },
    initAds() {
      setTimeout(() => {
        loadAd(5, 200);
      }, 500);
    }
  }
}

</script>
