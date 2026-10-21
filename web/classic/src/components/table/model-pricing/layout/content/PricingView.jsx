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

import React from 'react';
import { Button, Empty } from '@douyinfe/semi-ui';
import PricingTable from '../../view/table/PricingTable';
import PricingCardView from '../../view/card/PricingCardView';
import { getPricingRequestState } from '../../../../../hooks/model-pricing/pricing-request-state';

const PricingView = ({ viewMode = 'table', ...props }) => {
  const requestState = getPricingRequestState({
    loading: props.loading,
    error: Boolean(props.loadError),
    modelCount: props.filteredModels?.length ?? 0,
  });

  if (requestState === 'error') {
    return (
      <Empty
        title={props.t('价格暂时不可用，请稍后重试')}
        description={
          <Button theme='light' onClick={() => void props.refresh()}>
            {props.t('重试')}
          </Button>
        }
        style={{ minHeight: 320 }}
      />
    );
  }

  return viewMode === 'card' ? (
    <PricingCardView {...props} />
  ) : (
    <PricingTable {...props} />
  );
};

export default PricingView;
