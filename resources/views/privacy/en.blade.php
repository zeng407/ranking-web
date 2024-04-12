@extends('layouts.app', [
    'title' => __('Privacy Policy'),
    'ogImage' => asset('/storage/og-image.jpeg'),
    'stickyNav' => 'sticky-top',
])

@section('content')
    <div class="container">
        <div class="text-center mt-2">
            <h1>Privacy Policy</h1>
        </div>

        <p>We warmly welcome you to '{{ config('app.name') }}' (hereinafter referred to as this website). In order for you
            to use the various services and information of this website with peace of mind, we hereby explain the privacy
            policy of this website to protect your rights. Please read the following carefully:</p>

        <h3>I. Scope of Application of the Privacy Policy</h3>
        <ul>
            The content of the privacy policy, including how this website handles the personal identification information
            collected when you use the website services. The privacy policy does not apply to related linked websites
            outside of this website, nor does it apply to personnel not commissioned by this website or involved in
            management.
        </ul>

        <h3>II. Collection, Processing, and Use of Personal Data</h3>
        <ul>
            When you visit this website or use the services provided by this website, we will ask you to provide necessary
            personal information based on the nature of the service, and process and use your personal information within
            the scope of that specific purpose. Without your written consent, this website will not use personal data for
            other purposes.
            This website will retain your name, email address, contact information, and usage time when you use interactive
            features such as service mailboxes and surveys.
            During general browsing, the server will automatically record related behaviors, including the IP address of
            your connecting device, usage time, browser used, browsing and clicking data records, etc., as a reference for
            us to improve the website services. This record is for internal use and will not be disclosed to the public.
            To provide accurate services, we will statistically analyze the collected survey content. The statistical data
            or explanatory text of the analysis results will be published as needed for internal research, but will not
            involve specific personal data.
            You can request to correct or delete your account or the personal data collected by this website at any time.
            Please see the contact method at the bottom for contact information.

        </ul>

        <h3>III. Data Protection</h3>
        <ul>
            The servers of this website are equipped with firewalls, antivirus systems, and other related information
            security equipment and necessary security protection measures to protect the website and your personal data.
            Strict protection measures are adopted, and only authorized personnel can access your personal data. Relevant
            processing personnel have all signed confidentiality agreements. If there is a breach of confidentiality
            obligations, they will be subject to relevant legal penalties.
            If it is necessary to entrust other units to provide services due to business needs, this website will also
            strictly require them to comply with confidentiality obligations and take necessary inspection procedures to
            ensure that they will indeed comply.
        </ul>

        <h3>IV. External Links of the Website</h3>
        <ul>
            This website provides links to other websites. You can also click to enter other websites through the links
            provided by this website. However, the linked website does not apply to this website's privacy policy, and you
            must refer to the privacy policy of the linked website.
        </ul>

        <h3>V. Policy on Sharing Personal Data with Third Parties</h3>
        <ul>
            This website will never provide, exchange, rent or sell any of your personal data to other individuals, groups,
            private enterprises or public agencies, but those with a legal basis or contractual obligations are not limited
            to this.
            <br>
            The situations in the preceding paragraph include but are not limited to:
            <br>
            <ul>
                <li>With your written consent.</li>
                <li>Explicitly required by law.</li>
                <li>To avoid danger to your life, body, freedom or property.</li>
                <li>Cooperating with public agencies or academic research institutions based on public interest for
                    statistics or academic research, and the data is processed or collected by the provider in a way that
                    cannot identify a specific party.</li>
                <li>When your behavior on the website violates the terms of service or may damage or hinder the rights and
                    interests of the website and other users or cause damage to anyone, disclosing your personal data is
                    necessary for identification, contact or legal action.</li>
                <li>In your best interest.</li>
                <li>When this website entrusts a vendor to assist in collecting, processing or using your personal data, it
                    will duly supervise and manage the outsourced vendor or individual.</li>
            </ul>
        </ul>

        <h3>VI. Use of Cookies</h3>
        <ul>
            To provide you with the best service, our website will place and access our cookies on your computer. If you do
            not wish to accept the writing of cookies, you can set the privacy level to high in the browser functions you
            use, which will reject the writing of cookies, but it may lead to some functions of the website not working
            properly.
        </ul>

        <h3>VII. Amendment to the Privacy Policy</h3>
        <ul>This website's privacy policy will be amended at any time according to needs. The amended terms will be posted
            on the website.</ul>

        <h3>VIII. Contact Channel</h3>
        <ul>If you have any questions about the privacy policy of this site, or if you want to request changes or removal of
            personal data, please email to {{ config('app.contact_email') }}</ul>

    </div>
@endsection
@section('footer')
    @include('partial.footer')
@endsection
