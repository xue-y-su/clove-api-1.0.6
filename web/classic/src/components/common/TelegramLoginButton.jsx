/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import React, { useEffect, useRef } from 'react';
import PropTypes from 'prop-types';

const TelegramLoginButton = ({
  botName,
  buttonSize = 'large',
  cornerRadius,
  requestAccess = 'write',
  usePic = true,
  dataOnauth,
  dataAuthUrl,
  lang = 'en',
  className,
  children,
}) => {
  const containerRef = useRef(null);
  const onAuthRef = useRef(dataOnauth);

  useEffect(() => {
    onAuthRef.current = dataOnauth;
  }, [dataOnauth]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container || !botName) return undefined;

    const handleAuth = (user) => onAuthRef.current?.(user);
    const script = document.createElement('script');
    script.src = 'https://telegram.org/js/telegram-widget.js?22';
    script.async = true;
    script.setAttribute('data-telegram-login', botName);
    script.setAttribute('data-size', buttonSize);
    script.setAttribute('data-request-access', requestAccess);
    script.setAttribute('data-userpic', String(usePic));
    script.setAttribute('data-lang', lang);
    if (cornerRadius !== undefined) {
      script.setAttribute('data-radius', String(cornerRadius));
    }
    if (dataAuthUrl) {
      script.setAttribute('data-auth-url', dataAuthUrl);
    } else {
      window.TelegramLoginWidget = { dataOnauth: handleAuth };
      script.setAttribute(
        'data-onauth',
        'TelegramLoginWidget.dataOnauth(user)',
      );
    }
    container.appendChild(script);

    return () => {
      container.replaceChildren();
      if (window.TelegramLoginWidget?.dataOnauth === handleAuth) {
        delete window.TelegramLoginWidget;
      }
    };
  }, [
    botName,
    buttonSize,
    cornerRadius,
    dataAuthUrl,
    lang,
    requestAccess,
    usePic,
  ]);

  return (
    <>
      <div className={className} ref={containerRef} />
      {children}
    </>
  );
};

TelegramLoginButton.propTypes = {
  botName: PropTypes.string.isRequired,
  buttonSize: PropTypes.oneOf(['large', 'medium', 'small']),
  cornerRadius: PropTypes.number,
  requestAccess: PropTypes.string,
  usePic: PropTypes.bool,
  dataOnauth: PropTypes.func,
  dataAuthUrl: PropTypes.string,
  lang: PropTypes.string,
  className: PropTypes.string,
  children: PropTypes.node,
};

export default TelegramLoginButton;
