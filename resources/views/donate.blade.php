@extends('layouts.app', [
  'ogImage' => asset('/storage/og-image.jpeg'),
  'stickyNav' => 'sticky-top',
  'title' => __('Donate')
])

@section('content')

<div class="container my-5">
    @include('partial.lang', ['langPostfixURL' => url_path_without_locale()])
    <h1 class="text-center mb-4">{{__('Donate')}}</h1>
    <div class="row justify-content-center">
      <div class="col-12 col-lg-9 text-center my-2">
        <div class="text-left mb-5 font-size-large p-3 m-2 h-100" style="background: #b3a9a978">
          <h5>如果你喜歡這個網站，你可以考慮贊助我，讓我能持續改善網站功能</h5>
          <div class="p-2" style="background: rgb(255 246 206);">
            <p>贊助任意金額皆可獲得以下所有福利</p>
            <span>序號一組，包含：</span>
            <ul>
              <li>
                <b>30天無廣告權益</b>
              </li>
              <li>
                <b>30天感謝名單曝光</b>
              </li>
              <li>
                <b>30天會員VIP標記</b>
              </li>
            </ul>
          </div>

          <div class="mt-2">
          <p>註1：請付款時留下可聯絡的email，我會兩天內將序號透過email寄送給你</p>
          <p>註2：其他對於本站的建議可以寫在備註欄，感謝你的支持</p>
          <p>註3：本站使用歐付寶支付系統，透過<a href="https://payment.opay.tw/Broadcaster/Donate/EA6A62EF78EB2573D2570E271C1610B7">實況主贊助功能</a>贊助本站可免填資料
          </p>
          </div>
        </div>
      </div>
      <div class="col-12 col-lg-3">
        <div class="text-center p-3 m-2 h-100" style="border:solid 1px #000">
          <div>
            <h2>{{__('Payment')}}</h2>
            {{-- <img src="https://payment.opay.tw/Content/themes/WebStyle201404/images/allpay.png"/> --}}
          </div>
          <hr>
          <div class="text-center mt-2 text-black-50">
            <a target="_blank" href="https://p.opay.tw/bmlRV">
              <img src="https://i.imgur.com/v041xUS.png" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.coffee')}} ($50)</h5>
            </a>
          </div>
          <div class="text-center mt-2">
            <a target="_blank" href="https://p.opay.tw/l5Idq">
              <img src="https://i.imgur.com/n0tEcz8.jpeg" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.catfood')}} ($100)</h5>
            </a>
            <a href="#" data-toggle="modal" data-target="#catModal" @click.prevent>
              {{__('donate.see_the_cat')}}<i class="fas fa-search"></i>
            </a>
          </div>
          <div class="text-center mt-2">
            <a target="_blank" href="https://p.opay.tw/r8DGF">
              <img src="https://i.imgur.com/eMuh010.png" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.lunch')}} ($120)</h5>
            </a>
          </div>
          <div class="text-center mt-2">
            <a target="_blank" href="https://p.opay.tw/916Gs">
              <img src="https://i.imgur.com/pCq7eP4.jpeg" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.server')}} ($200)</h5>
            </a>
          </div>
          <div class="text-center mt-2">
            <a target="_blank" href="https://p.opay.tw/79Czd">
              <img src="https://i.imgur.com/2PXmGuk.png" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.whatever')}}</h5>
            </a>
          </div>
        </div>
      </div>
    </div>
</div>

<!-- Cat Modal -->
<div class="modal fade" id="catModal" tabindex="-1" aria-labelledby="catModalLabel" aria-hidden="true">
  <div class="modal-dialog">
    <div class="modal-content">
      <div class="modal-header">
        <button type="button" class="close" data-dismiss="modal" aria-label="Close">
          <span aria-hidden="true">&times;</span>
        </button>
      </div>
      <div class="modal-body align-self-center">
        <img src="https://i.imgur.com/unVRUTL.jpeg" class="img-fluid" alt="Cute Cat">
      </div>
    </div>
  </div>
</div>
@endsection


@section('footer')
  @include('partial.footer')
@endsection
