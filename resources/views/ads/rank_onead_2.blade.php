@if(!is_skip_ad())
<div id="oneadMFSDFPTag" data-ad-init="rank-onead-2"></div>
@push('scripts')
<script type="text/javascript">
  document.addEventListener('DOMContentLoaded', function() {
    var custom_call = function (params) {
      if (params.hasAd) {
        console.log('MFS has AD')
      } else {
        console.log('MFS AD empty')
      }
    }
    var _ONEAD = {};
    _ONEAD.pub = {};
    _ONEAD.pub.slotobj = document.getElementById("oneadMFSDFPTag");
    _ONEAD.pub.slots = ["div-onead-ad"];
    _ONEAD.pub.uid = "2000374";
    _ONEAD.pub.external_url = "https://onead.onevision.com.tw/";
    _ONEAD.pub.player_mode_div = "div-onead-ad";
    _ONEAD.pub.player_mode = "mobile-fullscreen";
    _ONEAD.pub.google_view_click = "%%CLICK_URL_UNESC%%";
    _ONEAD.pub.google_view_pixel = "";
    _ONEAD.pub.queryAdCallback = custom_call
    window.ONEAD_pubs = ONEAD_pubs || [];
    window.ONEAD_pubs.push(_ONEAD);

    var script = document.createElement('script');
    script.src = 'https://ad-specs.guoshipartners.com/static/js/onead-lib.min.js';
    document.body.appendChild(script);
  });
</script>
@endpush
@endif
