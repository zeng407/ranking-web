<div id = "oneadMFSDFPTag"></div>
<script type="text/javascript">
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
_ONEAD.pub.uid = "2000374"; //媒體 uid
_ONEAD.pub.player_mode_div = "div-onead-ad";
_ONEAD.pub.player_mode = "mobile-fullscreen";
_ONEAD.pub.google_view_click = "%%CLICK_URL_UNESC%%";
_ONEAD.pub.google_view_pixel = "";
_ONEAD.pub.mediaOption = {
    instantRemoveAfterClose : true
}
_ONEAD.pub.queryAdCallback = custom_call
var ONEAD_pubs = ONEAD_pubs || [];
ONEAD_pubs.push(_ONEAD);
</script>
<script type="text/javascript" src = "https://ad-specs.guoshipartners.com/static/js/onead-lib.min.js"></script>
