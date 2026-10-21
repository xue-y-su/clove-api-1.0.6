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

import { Banner } from '@douyinfe/semi-ui';
import PropTypes from 'prop-types';
import React from 'react';

import { getBalanceQueryErrorPresentation } from '../balance';

const BalanceQueryError = ({ code, retryAfterSeconds, t }) => {
  const presentation = getBalanceQueryErrorPresentation(
    code,
    retryAfterSeconds,
  );

  return (
    <Banner
      type={presentation.tone}
      title={t(presentation.titleKey)}
      description={t(presentation.descriptionKey, presentation.values)}
      className='!rounded-lg'
      closeIcon={null}
    />
  );
};

BalanceQueryError.propTypes = {
  code: PropTypes.string.isRequired,
  retryAfterSeconds: PropTypes.number,
  t: PropTypes.func.isRequired,
};

export default BalanceQueryError;
