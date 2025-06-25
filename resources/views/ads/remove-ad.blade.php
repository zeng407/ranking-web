@if (!is_skip_ad())
  <div id="remove-onead-ad-container" style="display: none;">
    <div id="div-onead-nd-01"></div>
    <div id="div-onead-draft-01"></div>
  </div>

  <script type="text/javascript">
    function getCookie(name) {
      const value = `; ${document.cookie}`;
      const parts = value.split(`; ${name}=`);
      if (parts.length === 2) return parts.pop().split(';').shift();
      return null;
    }

    function removeCookie(name) {
      document.cookie = name + '=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;';
    }

    var checkAdClick = function() {
      if (getCookie('_ad_click')) {
        // 建立提示元素
        const tip = document.createElement('div');
        tip.style.position = 'fixed';
        tip.style.left = '50%';
        tip.style.bottom = '32px';
        tip.style.transform = 'translateX(-50%)';
        tip.style.background = 'rgba(0,0,0,0.85)';
        tip.style.color = '#fff';
        tip.style.padding = '12px 24px';
        tip.style.borderRadius = '8px';
        tip.style.zIndex = 9999;
        tip.style.fontSize = '16px';
        tip.style.boxShadow = '0 2px 8px rgba(0,0,0,0.2)';
        tip.style.display = 'flex';
        tip.style.alignItems = 'center';
        tip.style.gap = '12px';
        tip.style.whiteSpace = 'nowrap';

        // 文字
        const text = document.createElement('span');
        text.textContent = '獎勵於重整後生效';

        // 按鈕
        const btn = document.createElement('button');
        btn.className = 'btn btn-secondary rounded-pill shadow';
        btn.onclick = function() {
          removeCookie('_ad_click');
          window.location.assign(window.location.href);
        };
        const icon = document.createElement('i');
        icon.className = 'fas fa-redo';
        icon.style.marginRight = '6px';

        btn.appendChild(icon);
        btn.appendChild(document.createTextNode('重新整理'));

        tip.appendChild(text);
        tip.appendChild(btn);
        document.body.appendChild(tip);

      }
    }
    checkAdClick();

    var custom_call = function(params) {
      if (params.hasAd) {
        console.log('Ad loaded successfully');
        window._onead_ad_loaded = true;
        // 動態插入右下角限時任務按鈕
        if (!document.getElementById('remove-ad-24hr-btn-container')) {
          var btnDiv = document.createElement('div');
          btnDiv.id = 'remove-ad-24hr-btn-container';
          btnDiv.style.position = 'fixed';
          btnDiv.style.right = '20px';
          btnDiv.style.bottom = '80px';
          btnDiv.style.zIndex = '2000';

          function getGradientColor(t) {
            // t: 1(綠) ~ 0(紅)
            const r = Math.round(227 * (1 - t) + 52 * t);
            const g = Math.round(52 * (1 - t) + 144 * t);
            const b = Math.round(47 * (1 - t) + 227 * t);
            return `rgb(${r},${g},${b})`;
          }

          var seconds = 60.0; // 倒數60秒
          var btn = document.createElement('button');
          btn.className = 'btn btn-primary rounded-pill shadow';
          btn.style.minWidth = '120px';

          // 建立 <i> icon
          const icon = document.createElement('i');
          icon.className = 'fa-solid fa-circle-question mr-1';

          // 建立文字節點
          const text = document.createTextNode(`限時任務 (${seconds.toFixed(1)})`);

          btn.appendChild(icon);
          btn.appendChild(text);
          btnDiv.appendChild(btn);
          document.body.appendChild(btnDiv);
          var timer = setInterval(function() {
            seconds -= 0.1;
            text.textContent = `限時任務 (${seconds.toFixed(1)})`;
            if (seconds <= 0) {
              clearInterval(timer);
              btnDiv.remove();
            } else {
              const t = seconds / 60; // 1 ~ 0
              btn.style.background = getGradientColor(t);
            }
          }, 100);
          btn.addEventListener('click', function() {
            showRemoveAdBackdrop();
            clearInterval(timer);
            btnDiv.remove();
          });
        }
      } else {
        console.log('No ad available at this time');
        window._onead_ad_loaded = false;
        var existBtn = document.getElementById('remove-ad-24hr-btn-container');
        if (existBtn) existBtn.remove();
      }
    }

    // 建立modal結構
    function ensureRemoveAdModal() {
      if (!document.getElementById('remove-ad-modal')) {
        var modalHtml = `
      <div class="modal fade" id="remove-ad-modal" tabindex="-1" role="dialog" aria-labelledby="removeAdModalLabel" aria-hidden="true">
        <div class="modal-dialog" role="document">
          <div class="modal-content">
            <div class="modal-header">
              <h5 class="modal-title" id="removeAdModalLabel">移除廣告 24 小時</h5>
              <button type="button" class="btn btn-outline-secondary" data-dismiss="modal" aria-label="Close">
                <span aria-hidden="true">&times;</span>放棄獎勵
              </button>
            </div>
            <div class="modal-body" id="remove-ad-modal-body">
              <div class="text-center mb-3" style="font-weight:bold;">⬇點擊廣告獲得獎勵⬇</div>
            </div>
          </div>
        </div>
      </div>`;
        var modalDiv = document.createElement('div');
        modalDiv.innerHTML = modalHtml;
        document.body.appendChild(modalDiv);
      }
    }

    // 顯示自訂 backdrop，蓋住全畫面並顯示 remove-onead-ad-container 及提示
    function showRemoveAdBackdrop() {
      if (document.getElementById('remove-ad-backdrop')) return;
      // 建立黯淡背景
      var backdrop = document.createElement('div');
      backdrop.id = 'remove-ad-backdrop';
      backdrop.style.position = 'fixed';
      backdrop.style.top = '0';
      backdrop.style.left = '0';
      backdrop.style.width = '100vw';
      backdrop.style.height = '100vh';
      backdrop.style.background = 'rgba(0,0,0,0.5)';
      backdrop.style.zIndex = '3000';
      backdrop.style.display = 'flex';
      backdrop.style.alignItems = 'center';
      backdrop.style.justifyContent = 'center';
      // 內容區塊
      var content = document.createElement('div');
      content.style.background = '#fff';
      content.style.borderRadius = '12px';
      content.style.boxShadow = '0 2px 16px rgba(0,0,0,0.2)';
      content.style.padding = '32px 24px 24px 24px';
      content.style.width = 'auto';
      content.style.height = 'auto';
      content.style.maxWidth = '95vw';
      content.style.maxHeight = '90vh';
      content.style.overflow = 'hidden';
      content.style.position = 'relative';
      // 關閉按鈕
      var closeBtn = document.createElement('button');
      closeBtn.innerHTML = '✕ 放棄獎勵';
      closeBtn.setAttribute('aria-label', 'Close');
      closeBtn.setAttribute('class', 'btn btn-outline-secondary d-block ml-auto mb-2');

      closeBtn.onclick = function() {
        backdrop.remove();
        // 關閉時也移除右下角按鈕
        var btnDiv = document.getElementById('remove-ad-24hr-btn-container');
        if (btnDiv) btnDiv.remove();
        setTimeout(function() {
          var anyModalOpen = document.querySelector('.modal.show, .modal[style*="display: block"]');
          if (anyModalOpen) {
            document.body.classList.add('modal-open');
          }
        }, 50);
      };
      content.appendChild(closeBtn);

      // 提示文字
      var tip = document.createElement('div');
      tip.id = 'remove-ad-tip';
      tip.innerText = '⬇點擊廣告獲得獎勵⬇「移除網頁所有廣告」';
      tip.style.textAlign = 'center';
      tip.style.fontWeight = 'bold';
      tip.style.marginBottom = '18px';
      content.appendChild(tip);

      // 將 remove-onead-ad-container 移入 backdrop 內容
      var adContainer = document.getElementById('remove-onead-ad-container');
      if (adContainer) {
        while (adContainer.firstChild) {
          content.appendChild(adContainer.firstChild);
        }

      }
      backdrop.appendChild(content);
      document.body.appendChild(backdrop);
    }

    ONEAD_TEXT = {};
    ONEAD_TEXT.pub = {};
    ONEAD_TEXT.pub.uid = "2000374";
    ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-draft-01");
    ONEAD_TEXT.pub.player_mode = "text-drive";
    ONEAD_TEXT.pub.queryAdCallback = custom_call;
    window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
    ONEAD_text_pubs.push(ONEAD_TEXT);

    ONEAD_TEXT = {};
    ONEAD_TEXT.pub = {};
    ONEAD_TEXT.pub.uid = "2000374";
    ONEAD_TEXT.pub.slotobj = document.getElementById("div-onead-nd-01");
    ONEAD_TEXT.pub.player_mode = "native-drive";
    ONEAD_TEXT.pub.player_mode_div = "div-onead-ad";
    ONEAD_TEXT.pub.max_threads = 3;
    ONEAD_TEXT.pub.position_id = /Mobi|Android|iPhone|iPad|iPod/i.test(navigator.userAgent) ? "5" : "0";
    ONEAD_TEXT.pub.queryAdCallback = custom_call;
    window.ONEAD_text_pubs = window.ONEAD_text_pubs || [];
    ONEAD_text_pubs.push(ONEAD_TEXT);

    // 廣告點擊監聽
    document.addEventListener('DOMContentLoaded', function() {
      var adContainer = document.getElementById('remove-onead-ad-container');

      if (adContainer) {
        var clickable = adContainer.querySelectorAll('a, div, iframe');
        clickable.forEach(function(el) {
          el.addEventListener('click', function handleAdClick(e) {
            e.preventDefault();

            fetch('/api/remove-ad-24hr', {
              method: 'POST',
              headers: {
                'X-Requested-With': 'XMLHttpRequest',
                'X-CSRF-TOKEN': document.querySelector('meta[name="csrf-token"]').getAttribute('content')
              }
            }).then(function() {
              function setCookie(name, value, hours) {
                let expires = "";
                if (hours) {
                  const date = new Date();
                  date.setTime(date.getTime() + (hours * 60 * 60 * 1000));
                  expires = "; expires=" + date.toUTCString();
                }
                document.cookie = name + "=" + encodeURIComponent(value) + expires + "; path=/";
              }
              setCookie('_ad_click', 1, 1);
              checkAdClick();
            });

            clickable.forEach(function(c) {
              c.removeEventListener('click', handleAdClick);
            });
          });
        });
      }
    });
  </script>
  <script src="https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js"></script>
  <script src="https://ad-specs.guoshipartners.com/static/js/ad-serv.min.js"></script>
@endif
