@if(!is_skip_ad())
<div id="remove-onead-ad-container">
  <div id="div-onead-nd-01"></div>
</div>
<script type="text/javascript">
  var custom_call = function (params) {
      if (params.hasAd) {
        console.log('Ad loaded successfully');
      } else {
        console.log('No ad available at this time');

      }
    }
  ONEAD_TEXT = {};
  ONEAD_TEXT.pub = {};
  ONEAD_TEXT.pub.uid = "2000374";
  ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-nd-01");
  ONEAD_TEXT.pub.player_mode = "native-drive";
  ONEAD_TEXT.pub.player_mode_div = "div-onead-ad";
  ONEAD_TEXT.pub.max_threads = 3;
  ONEAD_TEXT.pub.position_id = /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent)? "5" : "0";
  ONEAD_TEXT.pub.queryAdCallback = custom_call;
  window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
  ONEAD_text_pubs.push(ONEAD_TEXT);

  // 點擊廣告時發送 API，從 parent container 取得第一個子元素
  document.addEventListener('DOMContentLoaded', function() {
    var adDivContainer = document.getElementById('remove-onead-ad-container');
    var adDiv = adDivContainer ? adDivContainer.firstElementChild : null;
    if (adDiv) {
      console.log('Ad div found in remove-onead-ad-container');
      adDiv.addEventListener('click', function() {
        fetch('/api/remove-ad-24hr', { method: 'POST', headers: { 'X-Requested-With': 'XMLHttpRequest', 'X-CSRF-TOKEN': document.querySelector('meta[name="csrf-token"]').getAttribute('content') } });
      }, { once: true });
    }
  });
</script>
<script src="https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js"></script>
@endif
