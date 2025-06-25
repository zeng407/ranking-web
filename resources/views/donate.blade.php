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
      <div class="col-12 text-center my-2">
        <div class="text-left mb-5 font-size-large p-3 m-2 h-100" style="background: #b3a9a978">
          <h3>本網站由一名二選一愛好者獨立製作，如果你喜歡這個網站，可以考慮贊助我讓我能持續改善網站功能</h3>
          <p>任何想法或建議，歡迎在<a href="https://forms.gle/DfCfZGUjFncHJdN66" target="_blank">Google表單</a>提出</p>
        </div>
      </div>
      <div class="col-12">
        <div class="text-center p-3 m-2 h-100" style="border:solid 1px #000">
          <div>
            <h2>{{__('Payment')}}</h2>
            {{-- <img src="https://payment.opay.tw/Content/themes/WebStyle201404/images/allpay.png"/> --}}
          </div>
          <div class="row justify-content-center">
          <div class="col-6 text-center mt-2">
            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF">
              <img src="https://payment.ecpay.com.tw/Upload/QRCode/202407/QRCode_2ff3f451-50b1-4fe0-81f2-63ea255d817f.png">
              <h5 class="pt-1 text-nowrap">{{__('donate.ecpay')}} {{__('donate.whatever')}}</h5>
            </a>
          </div>
          <div class="col-6 text-center mt-2">
            <a target="_blank" href="https://qr.opay.tw/HTXLZ">
              <img src="https://payment.opay.tw/Upload/Broadcaster/1967663/QRcode/QRCode_EA6A62EF78EB2573D2570E271C1610B7.png" />
              <h5 class="pt-1 text-nowrap">{{__('donate.opay')}} {{__('donate.whatever')}}</h5>
            </a>
          </div>
          <hr>

          <div class="col-12 col-lg-3 text-center mt-2 text-black-50">
            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF">
              <img src="https://i.imgur.com/v041xUS.png" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.coffee')}} ($50)</h5>
            </a>
          </div>
          <div class="col-12 col-lg-3 text-center mt-2">
            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF">
              <img src="https://i.imgur.com/n0tEcz8.jpeg" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.catfood')}} ($100)</h5>
            </a>
            <a href="#" data-toggle="modal" data-target="#catModal" @click.prevent>
              {{__('donate.see_the_cat')}}<i class="fas fa-search"></i>
            </a>
          </div>
          <div class="col-12 col-lg-3 text-center mt-2">
            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF">
              <img src="https://i.imgur.com/eMuh010.png" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.lunch')}} ($120)</h5>
            </a>
          </div>
          <div class="col-12 col-lg-3 text-center mt-2">
            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF">
              <img src="https://i.imgur.com/pCq7eP4.jpeg" height="100px">
              <h5 class="pt-1 text-nowrap">{{__('donate.server')}} ($300)</h5>
            </a>
          </div>
          </div>
        </div>
      </div>
    </div>
</div>

<!-- Cat Modal -->
<div class="modal fade" id="catModal" tabindex="-1" aria-labelledby="catModalLabel" aria-hidden="true">
  <div class="modal-dialog">
    <div class="modal-content border-0" style="background: transparent;">
      <div class="modal-body align-self-center">
        <img src="{{asset('storage/cat.png')}}" class="img-fluid" alt="Cute Cat">
      </div>
    </div>
  </div>
</div>
@endsection


@section('footer')
  @include('partial.footer')
@endsection
