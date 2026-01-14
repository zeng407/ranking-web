@extends('layouts.app', [
  'ogImage' => asset('/storage/og-image.jpeg'),
  'stickyNav' => 'sticky-top',
  'title' => __('Donate')
])

@section('content')

<div class="container my-5 fade-in">
    @include('partial.lang', ['langPostfixURL' => url_path_without_locale()])

    <div class="row justify-content-center">
        <div class="col-lg-9">
            {{-- Header Section --}}
            <div class="text-center mb-5">
                <h1 class="font-weight-bold mb-3 display-4 text-dark">{{__('Donate')}}</h1>
                <div class="d-flex justify-content-center">
                    <div class="col-md-10">
                        <p class="lead text-muted mb-4" style="line-height: 1.8;">
                            本網站由一名二選一愛好者獨立製作，<br class="d-none d-md-block">
                            如果你喜歡這個網站，可以考慮贊助我讓我能持續改善網站功能。
                        </p>
                        <a href="https://forms.gle/DfCfZGUjFncHJdN66" target="_blank" class="btn btn-outline-secondary btn-sm rounded-pill px-4">
                            <i class="far fa-comment-dots mr-2"></i>任何想法或建議，歡迎提出
                        </a>
                    </div>
                </div>
            </div>

            {{-- Main Payment Options --}}
            <div class="card border-0 shadow-lg rounded-lg mb-5 overflow-hidden">
                <div class="card-header bg-white border-0 text-center py-4">
                    <h5 class="text-uppercase letter-spacing-2 text-primary font-weight-bold m-0">{{__('Payment')}}</h5>
                </div>
                <div class="card-body p-0">
                    <div class="row no-gutters">
                        <div class="col-md-6 border-right-md border-bottom border-bottom-md-0">
                            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF" class="d-block p-5 text-center text-decoration-none hover-bg-light transition-fast h-100">
                                <div class="mb-3">
                                    <img src="https://payment.ecpay.com.tw/Upload/QRCode/202407/QRCode_2ff3f451-50b1-4fe0-81f2-63ea255d817f.png"
                                         class="img-fluid rounded shadow-sm" style="max-width: 180px;">
                                </div>
                                <h5 class="text-dark font-weight-bold mb-1">{{__('donate.ecpay')}}</h5>
                                <span class="text-muted small">{{__('donate.whatever')}}</span>
                            </a>
                        </div>
                        <div class="col-md-6">
                            <a target="_blank" href="https://qr.opay.tw/HTXLZ" class="d-block p-5 text-center text-decoration-none hover-bg-light transition-fast h-100">
                                <div class="mb-3">
                                    <img src="https://payment.opay.tw/Upload/Broadcaster/1967663/QRcode/QRCode_EA6A62EF78EB2573D2570E271C1610B7.png"
                                         class="img-fluid rounded shadow-sm" style="max-width: 180px;" />
                                </div>
                                <h5 class="text-dark font-weight-bold mb-1">{{__('donate.opay')}}</h5>
                                <span class="text-muted small">{{__('donate.whatever')}}</span>
                            </a>
                        </div>
                    </div>
                </div>
            </div>

            {{-- Support Tiers --}}
            <div class="mb-5">
                <div class="text-center mb-4 position-relative">
                    <hr class="position-absolute w-100" style="top: 50%; opacity: 0.1; z-index: 0;">
                    <span class="d-inline-block bg-white px-3 position-relative text-muted font-weight-bold" style="z-index: 1;">小額贊助</span>
                </div>

                <div class="row">
                    {{-- Coffee --}}
                    <div class="col-6 col-md-3 mb-4">
                        <a target="_blank" href="https://p.ecpay.com.tw/677F5BF" class="card h-100 border-0 shadow-sm hover-lift text-decoration-none text-center p-3">
                            <div class="card-body p-2 d-flex flex-column align-items-center justify-content-between">
                                <div class="mb-3 ranking-icon-container">
                                    <img src="https://i.imgur.com/v041xUS.png" class="img-fluid" style="height: 60px; object-fit: contain;">
                                </div>
                                <div>
                                    <h6 class="text-dark font-weight-bold mb-1">{{__('donate.coffee')}}</h6>
                                    <span class="text-primary font-weight-bold">$50</span>
                                </div>
                            </div>
                        </a>
                    </div>

                    {{-- Cat Food --}}
                    <div class="col-6 col-md-3 mb-4">
                        <div class="card h-100 border-0 shadow-sm hover-lift text-center p-3 position-relative">
                            <a href="#" data-toggle="modal" data-target="#catModal" @click.prevent class="position-absolute text-muted" style="top: 10px; right: 10px;" title="{{__('donate.see_the_cat')}}">
                                <i class="fas fa-search"></i>
                            </a>
                            <a target="_blank" href="https://p.ecpay.com.tw/677F5BF" class="text-decoration-none d-block h-100">
                                <div class="card-body p-2 d-flex flex-column align-items-center justify-content-between h-100">
                                    <div class="mb-3 ranking-icon-container">
                                        <img src="https://i.imgur.com/n0tEcz8.jpeg" class="img-fluid rounded-circle shadow-sm" style="height: 60px; width: 60px; object-fit: cover;">
                                    </div>
                                    <div>
                                        <h6 class="text-dark font-weight-bold mb-1">{{__('donate.catfood')}}</h6>
                                        <span class="text-primary font-weight-bold">$100</span>
                                    </div>
                                </div>
                            </a>
                        </div>
                    </div>

                    {{-- Lunch --}}
                    <div class="col-6 col-md-3 mb-4">
                         <a target="_blank" href="https://p.ecpay.com.tw/677F5BF" class="card h-100 border-0 shadow-sm hover-lift text-decoration-none text-center p-3">
                            <div class="card-body p-2 d-flex flex-column align-items-center justify-content-between">
                                <div class="mb-3 ranking-icon-container">
                                    <img src="https://i.imgur.com/eMuh010.png" class="img-fluid" style="height: 60px; object-fit: contain;">
                                </div>
                                <div>
                                    <h6 class="text-dark font-weight-bold mb-1">{{__('donate.lunch')}}</h6>
                                    <span class="text-primary font-weight-bold">$120</span>
                                </div>
                            </div>
                        </a>
                    </div>

                    {{-- Server --}}
                    <div class="col-6 col-md-3 mb-4">
                         <a target="_blank" href="https://p.ecpay.com.tw/677F5BF" class="card h-100 border-0 shadow-sm hover-lift text-decoration-none text-center p-3">
                            <div class="card-body p-2 d-flex flex-column align-items-center justify-content-between">
                                <div class="mb-3 ranking-icon-container">
                                    <img src="https://i.imgur.com/pCq7eP4.jpeg" class="img-fluid" style="height: 60px; object-fit: contain;">
                                </div>
                                <div>
                                    <h6 class="text-dark font-weight-bold mb-1">{{__('donate.server')}}</h6>
                                    <span class="text-primary font-weight-bold">$300</span>
                                </div>
                            </div>
                        </a>
                    </div>
                </div>
            </div>
        </div>
    </div>
</div>

<!-- Cat Modal -->
<div class="modal fade" id="catModal" tabindex="-1" aria-labelledby="catModalLabel" aria-hidden="true">
  <div class="modal-dialog modal-dialog-centered">
    <div class="modal-content border-0 shadow-lg" style="background: rgba(255,255,255,0.9); backdrop-filter: blur(10px);">
      <div class="modal-body p-0">
        <button type="button" class="close position-absolute p-3" data-dismiss="modal" aria-label="Close" style="right: 0; z-index: 1;">
            <span aria-hidden="true">&times;</span>
        </button>
        <img src="{{asset('storage/cat.png')}}" class="img-fluid rounded" alt="Cute Cat" style="width: 100%;">
      </div>
    </div>
  </div>
</div>

<style>
    .letter-spacing-2 { letter-spacing: 2px; }
    .hover-bg-light:hover { background-color: #f8f9fa; }
    .transition-fast { transition: all 0.3s ease; }
    .hover-lift { transition: transform 0.2s ease, box-shadow 0.2s ease; }
    .hover-lift:hover { transform: translateY(-5px); box-shadow: 0 .5rem 1rem rgba(0,0,0,.15)!important; }
    @media (min-width: 768px) {
        .border-right-md { border-right: 1px solid #dee2e6; }
        .border-bottom-md-0 { border-bottom: 0 !important; }
    }
</style>
@endsection


@section('footer')
  @include('partial.footer')
@endsection
