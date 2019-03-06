from datetime import datetime, timedelta

from cryptography import x509
from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import dh, rsa
from mangle.common import config


KEY_SIZE = config.get("pki_key_size", 2048)
"""int: the size of generated keys.

Defines the number of bits that will be used when generating RSA private keys 
and Diffie-Hellman parameters. Read from the application settings but defaults
to 2048 bits.
"""


def certificate_authority():
    """
    Return's the application certificate authority keypair.
    :return: KeyPair
    """
    return load_keypair(
        crt=config.get("ca_crt"),
        key=config.get("ca_key"),
    )


def create_certificate_authority():
    """
    Creates and sets the application certificate authority keys.
    :return: None
    """
    crt, key = create_keypair("OpenVPN CA", 3650, False, True).pem()
    config.set("ca_crt", crt)
    config.set("ca_key", key)


def create_client_keypair(name, days):
    """
    Returns a new client keypair.
    :return: KeyPair
    """
    return create_keypair(name, days, False, False)


def create_server_keypair(name, days):
    """
    Returns a new server keypair.
    :return: KeyPair
    """
    return create_keypair(name, days, True, False)


def create_keypair(name, days, is_server, is_ca):
    """
    Returns a new KeyPair.
    :return: KeyPair
    """
    private_key = rsa.generate_private_key(65537, KEY_SIZE, default_backend())

    builder = x509.CertificateBuilder(
        serial_number=x509.random_serial_number(),
        not_valid_before=datetime.now() - timedelta(days=1),
        not_valid_after=datetime.now() + timedelta(days=days),
        public_key=private_key.public_key(),
        extensions=_get_certificate_extensions(is_server, is_ca),
        subject_name=x509.Name([
            x509.NameAttribute(x509.NameOID.COMMON_NAME, name),
        ]),
    )

    # if this is a CA certificate then this will be self-signed and they signer
    # is set to a new Keypair that includes the builder certificate and new
    # private key. Otherwise, set the signer to the application's CA Keypair
    if is_ca:
        signer = KeyPair(builder, private_key)
        builder = builder.issuer_name(builder._subject_name)
    else:
        signer = certificate_authority()
        builder = builder.issuer_name(signer.crt.subject)

    # sign the certificate with the signer private key and return a new KeyPair
    certificate = builder.sign(
        private_key=signer.key,
        algorithm=hashes.SHA512(),
        backend=default_backend()
    )
    return KeyPair(certificate, private_key)


def create_dh_params(size=None):
    """
    Returns a new set of Diffie-Hellman parameters of the given size. If the
    ``size`` arg is not specified, then the default KEYSIZE is used.
    :return: str
    """
    if not size:
        size = KEY_SIZE

    params = dh.generate_parameters(2, size, default_backend())
    return encode_dh_params(params)


def create_crl(*serials):
    """
    Returns a new certificate revocation list that contains all of the revoked
    certificates with the given list of serial numbers.
    :return: str
    """
    signer = certificate_authority()

    builder = x509.CertificateRevocationListBuilder(
        last_update=datetime.now() - timedelta(days=1),
        next_update=datetime.now() + timedelta(days=3650),
        issuer_name=signer.crt.subject,
    )

    # iterate over each serial number and create a new revoked certificate
    # that is added to the CRL builder
    for serial in serials:
        revoked = x509.RevokedCertificateBuilder(
            serial_number=int(serial),
            revocation_date=datetime.now() - timedelta(days=1),
        )
        revoked = revoked.build(default_backend())
        builder = builder.add_revoked_certificate(revoked)

    # sign the CRL with the certificate authority private key
    crl = builder.sign(
        private_key=signer.key,
        algorithm=hashes.SHA512(),
        backend=default_backend(),
    )
    return encode_certificate(crl)


def _get_certificate_extensions(is_server, is_ca):
    """
    Returns a list that contains all of the certificate extensions based on the
    certificate type.
    :return: List[x509.Extension]
    """
    extensions = []

    # path length for CA is 0
    path_length = None
    if is_ca:
        path_length = 0

    # BasicConstraints
    extensions.append(x509.Extension(
        oid=x509.BasicConstraints.oid,
        critical=True,
        value=x509.BasicConstraints(is_ca, path_length)
    ))

    # KeyUsage
    extensions.append(x509.Extension(
        oid=x509.KeyUsage.oid,
        critical=True,
        value=x509.KeyUsage(
            digital_signature=True,
            content_commitment=False,
            key_encipherment=is_server,
            data_encipherment=False,
            key_agreement=True,
            key_cert_sign=is_ca,
            crl_sign=is_ca,
            encipher_only=False,
            decipher_only=False
        )
    ))

    # add server certificate and client certificate specific extensions
    if is_server:
        extensions.append(x509.Extension(
            oid=x509.ExtendedKeyUsage.oid,
            critical=True,
            value=x509.ExtendedKeyUsage([
                x509.ExtendedKeyUsageOID.SERVER_AUTH,
            ])
        ))
    elif not is_ca:
        # if this is not a server certificate and not a certificate autority
        # certificate then it's assumed to be a client certificate
        extensions.append(x509.Extension(
            oid=x509.ExtendedKeyUsage.oid,
            critical=True,
            value=x509.ExtendedKeyUsage([
                x509.ExtendedKeyUsageOID.CLIENT_AUTH,
            ])
        ))
    return extensions


def parse_certificate(crt):
    """
    Returns a certificate object parsed from the given PEM string.
    :return: x509.Certificate
    """
    return x509.load_pem_x509_certificate(
        data=bytes(crt, "utf-8"),
        backend=default_backend()
    )


def parse_private_key(key, password=None):
    """
    Returns a private key object parsed from the given PEM string. If the
    private key requires a password, then the given password value is used.
    :return: rsa.PrivateKey
    """
    return serialization.load_pem_private_key(
        data=bytes(key, "utf-8"),
        password=password,
        backend=default_backend()
    )


def encode_certificate(certificate):
    """
    Returns a PEM string encoded from the given certificate object.
    :return: str
    """
    data = certificate.public_bytes(serialization.Encoding.PEM)
    return str(data, "utf-8")


def encode_private_key(private_key):
    """
    Returns a PEM string encoded from the given RSA private key object.
    :return: str
    """
    data = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.TraditionalOpenSSL,
        encryption_algorithm=serialization.NoEncryption()
    )
    return str(data, "utf-8")


def encode_dh_params(params):
    """
    Returns the string value of the given Diffie-Hellman parameters.
    :return: str
    """
    data = params.parameter_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.ParameterFormat.PKCS3,
    )
    return str(data, "utf-8")


#######################################
# KeyPair
#######################################

class KeyPair:
    """
    A KeyPair contains an X509 certificate and it's RSA private key objects and
    defines utility methods for
    """
    def __init__(self, crt, key):
        self.crt = crt
        self.key = key

    @property
    def fingerprint(self):
        """
        Returns the SHA1 fingerprint of the certificate.
        :return: str
        """
        value = self.crt.fingerprint(hashes.SHA1())
        return ":".join("{:02x}".format(v) for v in value)

    @property
    def certificate_pem(self):
        """
        Returns the certificate PEM string.
        :return: str
        """
        return encode_certificate(self.crt)

    @property
    def private_key_pem(self):
        """
        Returns the private key PEM string.
        :return: str
        """
        return encode_private_key(self.key)

    def pem(self):
        """
        Returns a tuple containing the certificate and private key PEM strings.
        :return: Tuple[str,str]
        """
        return self.certificate_pem, self.private_key_pem


def load_keypair(crt, key):
    """
    Returns a KeyPair from the given certificate and private key PEM strings.
    :return: KeyPair
    """
    return KeyPair(parse_certificate(crt), parse_private_key(key))
