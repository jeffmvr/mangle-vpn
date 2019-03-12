import os

from mangle.common.utils import strings


# Build paths inside the project like this: os.path.join(BASE_DIR, ...)
BASE_DIR = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


# Quick-start development settings - unsuitable for production
# See https://docs.djangoproject.com/en/2.1/howto/deployment/checklist/

# SECURITY WARNING: don't run with debug turned on in production!
DEBUG = os.environ.get("MANGLE_DEBUG", False)

ALLOWED_HOSTS = ['*']

AUTHENTICATION_BACKENDS = [
    "mangle.web.authentication.oauth2.backend.OAuth2Backend",
]

# Application definition

INSTALLED_APPS = [
    'django.contrib.auth',
    'django.contrib.contenttypes',
    'django.contrib.sessions',
    'huey.contrib.djhuey',
    'mangle.cli',
    'mangle.common',
    'mangle.web',
]

MIDDLEWARE = [
    'django.middleware.security.SecurityMiddleware',
    'django.contrib.sessions.middleware.SessionMiddleware',
    'django.middleware.common.CommonMiddleware',
    'django.middleware.csrf.CsrfViewMiddleware',
    'django.contrib.auth.middleware.AuthenticationMiddleware',
    'django.middleware.clickjacking.XFrameOptionsMiddleware',
    'mangle.web.middleware.config_middleware',
]

ROOT_URLCONF = 'mangle.web.urls'

AUTH_USER_MODEL = 'common.User'

TEMPLATES = [
    {
        'BACKEND': 'django.template.backends.django.DjangoTemplates',
        'DIRS': [
            os.path.join(BASE_DIR, "ui", "public"),
        ],
        'APP_DIRS': True,
        'OPTIONS': {
            'context_processors': [
                'django.template.context_processors.debug',
                'django.template.context_processors.request',
                'django.contrib.auth.context_processors.auth',
                'django.contrib.messages.context_processors.messages',
            ],
        },
    },
]

WSGI_APPLICATION = 'mangle.wsgi.application'

# Password validation
# https://docs.djangoproject.com/en/2.1/ref/settings/#auth-password-validators

AUTH_PASSWORD_VALIDATORS = [
    {
        'NAME': 'django.contrib.auth.password_validation.UserAttributeSimilarityValidator',
    },
    {
        'NAME': 'django.contrib.auth.password_validation.MinimumLengthValidator',
    },
    {
        'NAME': 'django.contrib.auth.password_validation.CommonPasswordValidator',
    },
    {
        'NAME': 'django.contrib.auth.password_validation.NumericPasswordValidator',
    },
]


# Internationalization
# https://docs.djangoproject.com/en/2.1/topics/i18n/

LANGUAGE_CODE = 'en-us'

TIME_ZONE = 'UTC'

USE_I18N = True

USE_L10N = True

USE_TZ = True

#######################################
# Application Paths
#######################################

# Directories
DATA_DIR = os.path.join(BASE_DIR, "data")
KEY_DIR = os.path.join(DATA_DIR, "keys")
LOG_DIR = os.path.join(DATA_DIR, "logs")
SYSTEMD_DIR = os.path.join(DATA_DIR, "systemd")

# Files
DATABASE_PATH = os.path.join(DATA_DIR, "mangle.db")
DJANGO_LOG_FILE = os.path.join(LOG_DIR, "django.log")
OPENVPN_CONFIG_FILE = os.path.join(DATA_DIR, "openvpn.conf")
OPENVPN_LOG_FILE = os.path.join(LOG_DIR, "openvpn.log")
OPENVPN_MANAGEMENT_SOCKET = "/run/mangle-vpn.sock"
OPENVPN_STATUS_FILE = os.path.join(LOG_DIR, "openvpn-status.log")
PKI_CRL_FILE = os.path.join(KEY_DIR, "crl.pem")
SECRET_KEY_FILE = os.path.join(KEY_DIR, "secret.key")
SYSTEMD_VPN_FILE = os.path.join(SYSTEMD_DIR, "mangle-vpn.service")
SYSTEMD_WEB_FILE = os.path.join(SYSTEMD_DIR, "mangle-web.service")
SYSTEMD_TASKS_FILE = os.path.join(SYSTEMD_DIR, "mangle-tasks.service")
TASKS_LOG_FILE = os.path.join(LOG_DIR, "tasks.log")
WEB_ACCESS_LOG_FILE = os.path.join(LOG_DIR, "wsgi-access.log")
WEB_ERROR_LOG_FILE = os.path.join(LOG_DIR, "wsgi-error.log")
WEB_SSL_CRT_FILE = os.path.join(KEY_DIR, "ssl.crt")
WEB_SSL_KEY_FILE = os.path.join(KEY_DIR, "ssl.key")
WEB_SSL_DH_FILE = os.path.join(KEY_DIR, "ssl.dh")
WEB_VHOST_FILE = "/etc/nginx/conf.d/mangle.conf"
WEB_WSGI_SOCKET = "/run/mangle-web.sock"

#######################################
# Database
# https://docs.djangoproject.com/en/2.1/ref/settings/#databases
#######################################

DATABASES = {
    'default': {
        'ENGINE': 'django.db.backends.sqlite3',
        'NAME': DATABASE_PATH,
    }
}

#######################################
# Django REST Framework
# https://www.django-rest-framework.org/
#######################################

REST_FRAMEWORK = {
    'DEFAULT_RENDERER_CLASSES': (
        'rest_framework.renderers.JSONRenderer',
    ),
    'DEFAULT_PAGINATION_CLASS': 'mangle.web.api.pagination.ApiPagination',
}

#######################################
# Logging
# https://docs.djangoproject.com/en/2.1/topics/logging/
#######################################

LOGGING = {
    'version': 1,
    'disable_existing_loggers': False,
    'formatters': {
        'standard': {
            'format': '[%(asctime)s: %(levelname)s/%(name)s] %(message)s'
        },
    },
    'handlers': {
        'default': {
            'level': 'INFO',
            'class': 'logging.handlers.RotatingFileHandler',
            'formatter': 'standard',
            'filename': DJANGO_LOG_FILE,
            'maxBytes': 1024*1024*10,       # 10MB
            'backupCount': 10,
        },
        'tasks': {
            'level': 'INFO',
            'class': 'logging.handlers.RotatingFileHandler',
            'formatter': 'standard',
            'filename': TASKS_LOG_FILE,
            'maxBytes': 1024*1024*10,       # 10MB
            'backupCount': 10,
        }
    },
    'loggers': {
        '': {
            'handlers': ['default', ],
            'level': 'INFO',
            'propagate': False,
        },
        'mangle.common.tasks': {
            'handlers': ['tasks', ],
            'level': 'INFO',
            'propagate': False,
        },
        'huey.consumer': {
            'handlers': ['tasks', ],
            'level': 'INFO',
            'propagate': False,
        },
    },
}

#######################################
# Security
# https://docs.djangoproject.com/en/2.1/topics/security/
#######################################

CSRF_COOKIE_SECURE = True
SECURE_BROWSER_XSS_FILTER = True
SECURE_CONTENT_TYPE_NOSNIFF = True
SESSION_COOKIE_AGE = 3600
SESSION_COOKIE_HTTPONLY = True
SESSION_COOKIE_SECURE = True
SESSION_EXPIRE_AT_BROWSER_CLOSE = True
X_FRAME_OPTIONS = 'DENY'

# SECRET_KEY is read from file only
with open(SECRET_KEY_FILE, "r") as f:
    SECRET_KEY = f.read()
