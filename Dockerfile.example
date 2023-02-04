FROM ubuntu:22.04

ENV TZ=Asia/Taipei
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && \
    echo $TZ > /etc/timezone
ENV USER_NAME ranker

RUN apt-get update && apt-get install -y \
    software-properties-common

RUN add-apt-repository main -y && \
    add-apt-repository ppa:ondrej/php -y

RUN apt-get update && apt-get install -y \
    mysql-client \
    apache2 \
    git \
    supervisor \
    cron \
    zsh \
    sudo \
    nano \
    vim \
    unzip \
    php8.0-dev \
    php8.0 \
    php8.0-bcmath \
    php8.0-xml \
    php8.0-curl \
    php8.0-iconv \
    php8.0-gd \
    php8.0-mysql \
    php8.0-mcrypt \
    php8.0-intl \
    php8.0-mbstring \
    php8.0-pcov \
    php8.0-zip \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*

# conigure apache
COPY dockerfiles/apache /etc/apache2/sites-available/
RUN a2dissite 000-default.conf && \
    a2ensite rk-web && \
    a2enmod php8.0 && \
    a2enmod rewrite

# install composer
COPY --from=composer /usr/bin/composer /usr/bin/composer

# install phpredis
RUN pecl install redis && \
    echo "extension=redis.so" >> /etc/php/8.0/apache2/php.ini && \
    echo "extension=redis.so" >> /etc/php/8.0/cli/php.ini

# conigure supervisor
COPY dockerfiles/supervisors/supervisord.conf /etc/supervisor/conf.d/supervisord.conf

# conigure crontab
RUN echo '* * * * * php /var/www/rk-web/artisan schedule:run >> /dev/null 2>&1' > /var/spool/cron/crontabs/www-data && \
    crontab -u www-data /var/spool/cron/crontabs/www-data

# switch user
RUN useradd -ms /bin/bash -g www-data -G sudo $USER_NAME && \
    echo "$USER_NAME ALL=(ALL) NOPASSWD: ALL" >> /etc/sudoers && \
    umask 002
USER $USER_NAME

# install zsh
COPY dockerfiles/zsh/* /home/$USER_NAME/
RUN curl -L git.io/antigen > ~/antigen.zsh && \
    /bin/zsh ~/.zshrc && \
    echo "exec zsh" >> ~/.bashrc

WORKDIR /var/www/rk-web

CMD ["sudo", "/usr/bin/supervisord"]
