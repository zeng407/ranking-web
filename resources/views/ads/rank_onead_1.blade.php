<div id="div-onead-nd-01"></div>
  <script type="text/javascript">
    var custom_call = function (params) {
        if (params.hasAd) {
          console.log('ND has AD')
        } else {
          console.log('ND AD Empty');
        }
      }

    ONEAD_TEXT = {};
    ONEAD_TEXT.pub = {};
    ONEAD_TEXT.pub.uid = 2000374; //媒體 uid
    ONEAD_TEXT.pub.player_mode_div = "div-onead-ad";
    ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-nd-01");
    ONEAD_TEXT.pub.player_mode = "native-drive";
    ONEAD_TEXT.pub.max_threads = 3;
    ONEAD_TEXT.pub.position_id = /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent)? "5" : "0";
    ONEAD_TEXT.pub.mediaOption = { landingNewPage : true }
    ONEAD_TEXT.pub.queryAdCallback = custom_call
    window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
    ONEAD_text_pubs.push(ONEAD_TEXT);
  </script>

  <script src="https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js"></script>
